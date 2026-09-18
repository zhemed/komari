package dockerapi

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// 真实的 /proc/self/mountinfo 片段（containerd 快照器 + bind 挂载；2026-09-18 取自 host 网络容器）。
const sampleMountInfo = `1382 1366 0:28 / / rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/588/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/589/fs
1390 1382 0:30 /opt/docker/komari/data /app/data rw,relatime - ext4 /dev/sda2 rw,errors=remount-ro
1391 1382 0:30 /var/run/docker.sock /var/run/docker.sock rw,relatime - ext4 /dev/sda2 rw,errors=remount-ro
1392 1382 0:30 /etc/localtime /etc/localtime ro,relatime - ext4 /dev/sda2 ro,errors=remount-ro
1393 1382 0:32 / /proc rw,nosuid - proc proc rw
`

func TestParseBindMountsSkipsRootMounts(t *testing.T) {
	pairs := parseBindMounts(sampleMountInfo)
	if len(pairs) != 3 {
		t.Fatalf("应解析出 3 条 bind 挂载（数据目录/socket/localtime），实际 %d：%+v", len(pairs), pairs)
	}
	// 容器自己那层 overlay 的 root 是 "/"，必须被排除
	for _, p := range pairs {
		if p.HostRoot == "/" || p.HostRoot == "" {
			t.Fatalf("混进了非 bind 挂载：%+v", p)
		}
	}
	want := mountPair{MountPoint: "/app/data", HostRoot: "/opt/docker/komari/data"}
	found := false
	for _, p := range pairs {
		if p == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("缺少 %+v：%+v", want, pairs)
	}
}

func TestMatchSelfContainerPicksContainerWithMatchingBind(t *testing.T) {
	self := []mountPair{
		{MountPoint: "/app/data", HostRoot: "/opt/docker/komari/data"},
		{MountPoint: "/var/run/docker.sock", HostRoot: "/var/run/docker.sock"},
	}
	items := []ContainerSummary{
		{ID: "other-with-only-socket", Mounts: []Mount{{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock"}}},
		{ID: "me", Mounts: []Mount{
			{Type: "bind", Source: "/opt/docker/komari/data", Destination: "/app/data"},
			{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock"},
		}},
		{ID: "another-project", Mounts: []Mount{{Type: "volume", Source: "/var/lib/docker/volumes/x/_data", Destination: "/app/data"}}},
	}
	if got := matchSelfContainer(self, items); got != "me" {
		t.Fatalf("应匹配挂载对最多的容器 me，实际 %q", got)
	}
}

func TestMatchSelfContainerReturnsEmptyWhenNoMatch(t *testing.T) {
	self := []mountPair{{MountPoint: "/app/data", HostRoot: "/opt/docker/komari/data"}}
	items := []ContainerSummary{
		{ID: "a", Mounts: []Mount{{Type: "bind", Source: "/srv/other", Destination: "/app/data"}}},
		{ID: "b", Mounts: []Mount{{Type: "bind", Source: "/opt/docker/komari/data", Destination: "/somewhere-else"}}},
	}
	if got := matchSelfContainer(self, items); got != "" {
		t.Fatalf("挂载点或目标不一致时不该匹配，实际 %q", got)
	}
}

// 反向用例：把"目标路径也要一致"这条去掉后，b 容器会被误判成自己。
func TestMatchSelfContainerRequiresDestinationMatch(t *testing.T) {
	self := []mountPair{{MountPoint: "/app/data", HostRoot: "/opt/docker/komari/data"}}
	items := []ContainerSummary{
		{ID: "same-source-wrong-destination", Mounts: []Mount{{Type: "bind", Source: "/opt/docker/komari/data", Destination: "/mnt/data"}}},
	}
	if got := matchSelfContainer(self, items); got == "same-source-wrong-destination" {
		t.Fatalf("只比对 Source 会误判：%q", got)
	}
}

// 回归测试：ListContainers 必须命中真实的版本化路径。
// 曾经把整条 URL 传给 do()（它内部还会再拼一次 /v1.xx 前缀），结果拼成
// http://docker/v1.41http://docker/... → 404，导致 host 网络下识别自身容器静默失败。
func TestListContainersHitsVersionedPath(t *testing.T) {
	client, fake := newTestClient(t)
	fake.listItems = []ContainerSummary{
		{ID: "me", Mounts: []Mount{{Type: "bind", Source: "/srv/komari/data", Destination: "/app/data"}}},
	}
	items, err := client.ListContainers(context.Background())
	if err != nil {
		t.Fatalf("列出容器失败：%v", err)
	}
	if len(items) != 1 || items[0].ID != "me" {
		t.Fatalf("列表内容不对：%+v", items)
	}
	if len(items[0].Mounts) != 1 || items[0].Mounts[0].Source != "/srv/komari/data" {
		t.Fatalf("Mounts 没解析出来：%+v", items[0].Mounts)
	}
	if !fake.called("list:all=1") {
		t.Fatalf("没有带 all=1 查询参数，调用记录：%v", fake.calls)
	}
}

// 整链测试：host 网络下 hostname/cgroup 都没线索时，靠 bind 挂载比对认出自己。
func TestSelfContainerIDFallsBackToMountMatching(t *testing.T) {
	client, fake := newTestClient(t)
	fake.listItems = []ContainerSummary{
		{ID: "someone-else", Mounts: []Mount{{Type: "bind", Source: "/srv/other/data", Destination: "/app/data"}}},
		{ID: "this-is-me", Mounts: []Mount{
			{Type: "bind", Source: "/srv/komari/data", Destination: "/app/data"},
			{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock"},
		}},
	}
	// 用临时文件冒充 /proc/self/mountinfo：数据目录挂载点与上面第二个容器对得上。
	mountinfo := "1 0 8:2 / / rw - overlay overlay rw\n" +
		"2 1 8:2 /srv/komari/data /app/data rw - ext4 /dev/sda2 rw\n" +
		"3 1 8:2 /var/run/docker.sock /var/run/docker.sock rw - ext4 /dev/sda2 rw\n"
	path := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(path, []byte(mountinfo), 0o644); err != nil {
		t.Fatalf("写临时 mountinfo 失败：%v", err)
	}
	prev := selfMountInfoPath
	selfMountInfoPath = path
	t.Cleanup(func() { selfMountInfoPath = prev })

	// hostname/cgroup 无线索（测试进程的 hostname 不是 12 位十六进制；cgroup 也不是 64 位 ID）
	id, err := client.SelfContainerID(context.Background())
	if err != nil {
		t.Fatalf("自识别失败：%v", err)
	}
	if id != "this-is-me" {
		t.Fatalf("认错容器：%q", id)
	}
}

// 没有任何容器与之匹配时必须报错（调用方据此回落，而不是猜一个容器去重建）。
func TestSelfContainerIDErrorsWhenNothingMatches(t *testing.T) {
	client, fake := newTestClient(t)
	fake.listItems = []ContainerSummary{
		{ID: "someone-else", Mounts: []Mount{{Type: "bind", Source: "/srv/other/data", Destination: "/app/data"}}},
	}
	path := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(path, []byte("2 1 8:2 /srv/komari/data /app/data rw - ext4 /dev/sda2 rw\n"), 0o644); err != nil {
		t.Fatalf("写临时 mountinfo 失败：%v", err)
	}
	prev := selfMountInfoPath
	selfMountInfoPath = path
	t.Cleanup(func() { selfMountInfoPath = prev })

	if _, err := client.SelfContainerID(context.Background()); err == nil {
		t.Fatalf("匹配不上时应返回错误")
	}
}
