package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleCompose = `services:
  komari:
    image: ghcr.io/zhemed/komari:0.0.17   # ① 钉版本
    container_name: komari
    restart: unless-stopped
    volumes:
      - ./data:/app/data
  other:
    image: nginx:1.27
`

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yaml")
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatalf("准备临时 compose 文件失败：%v", err)
	}
	return path
}

func TestSyncComposeImageRewritesTagAndKeepsComment(t *testing.T) {
	path := writeTemp(t, sampleCompose)
	res, err := SyncComposeImage(path, "komari", "ghcr.io/zhemed/komari:0.0.18")
	if err != nil {
		t.Fatalf("同步返回错误：%v", err)
	}
	if !res.Changed {
		t.Fatalf("应改写文件，实际跳过：%s", res.Reason)
	}
	if res.OldImage != "ghcr.io/zhemed/komari:0.0.17" || res.NewImage != "ghcr.io/zhemed/komari:0.0.18" {
		t.Fatalf("新旧值不对：%q → %q", res.OldImage, res.NewImage)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "image: ghcr.io/zhemed/komari:0.0.18   # ① 钉版本") {
		t.Fatalf("改写后没保留缩进/注释：\n%s", got)
	}
	// 其它 service 与其它行必须原样
	if !strings.Contains(string(got), "image: nginx:1.27") {
		t.Fatalf("误改了别的 service：\n%s", got)
	}
	if !strings.Contains(string(got), "container_name: komari") || !strings.Contains(string(got), "./data:/app/data") {
		t.Fatalf("改动了 service 块内其它行：\n%s", got)
	}
	// 备份必须存在且等于改动前的内容
	bak, err := os.ReadFile(res.BackupPath)
	if err != nil {
		t.Fatalf("备份不可读：%v", err)
	}
	if string(bak) != sampleCompose {
		t.Fatalf("备份内容不是改写前的原文")
	}
	// 权限保留（0640）
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("文件权限被改掉：%v", info.Mode().Perm())
	}
}

func TestSyncComposeImageIsIdempotent(t *testing.T) {
	path := writeTemp(t, sampleCompose)
	before, _ := os.Stat(path)
	res, err := SyncComposeImage(path, "komari", "ghcr.io/zhemed/komari:0.0.17")
	if err != nil {
		t.Fatalf("同步返回错误：%v", err)
	}
	if res.Changed {
		t.Fatalf("tag 已一致时不该改写文件")
	}
	after, _ := os.Stat(path)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("tag 已一致时文件被触碰")
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("tag 已一致时不该产生备份")
	}
}

func TestSyncComposeImageSkipsVariableAndMissingService(t *testing.T) {
	path := writeTemp(t, "services:\n  komari:\n    image: ghcr.io/zhemed/komari:${TAG}\n")
	res, err := SyncComposeImage(path, "komari", "ghcr.io/zhemed/komari:0.0.18")
	if err != nil || res.Changed {
		t.Fatalf("变量形式应跳过：changed=%v err=%v", res.Changed, err)
	}
	if !strings.Contains(res.Reason, "变量") {
		t.Fatalf("跳过原因应说明是变量：%q", res.Reason)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "${TAG}") {
		t.Fatalf("变量文件被改写：%s", got)
	}

	path2 := writeTemp(t, sampleCompose)
	res2, err := SyncComposeImage(path2, "nope", "ghcr.io/zhemed/komari:0.0.18")
	if err != nil || res2.Changed {
		t.Fatalf("找不到 service 应跳过：changed=%v err=%v", res2.Changed, err)
	}
	if !strings.Contains(res2.Reason, "找不到 service") {
		t.Fatalf("原因不对：%q", res2.Reason)
	}
}

func TestSyncComposeImageQuotedValue(t *testing.T) {
	path := writeTemp(t, "services:\n  komari:\n    image: \"ghcr.io/zhemed/komari:0.0.17\"\n")
	res, err := SyncComposeImage(path, "komari", "ghcr.io/zhemed/komari:0.0.18")
	if err != nil || !res.Changed {
		t.Fatalf("带引号的值也应能改：changed=%v err=%v reason=%s", res.Changed, err, res.Reason)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), `image: "ghcr.io/zhemed/komari:0.0.18"`) {
		t.Fatalf("引号丢失：%s", got)
	}
}

func TestSyncComposeImageKeepsRepositoryFromFile(t *testing.T) {
	// 私有 registry / 镜像加速：只换 tag，保留文件里的仓库名
	path := writeTemp(t, "services:\n  komari:\n    image: registry.example.com:5000/mirror/komari:0.0.17\n")
	res, err := SyncComposeImage(path, "komari", "ghcr.io/zhemed/komari:0.0.18")
	if err != nil || !res.Changed {
		t.Fatalf("应改写：changed=%v err=%v", res.Changed, err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "registry.example.com:5000/mirror/komari:0.0.18") {
		t.Fatalf("仓库名被改掉：%s", got)
	}
}

func TestSyncComposeImageServiceBlockBoundary(t *testing.T) {
	// 目标 service 在前，后面紧跟另一个 service 的 image：不能改错块
	content := "services:\n  komari:\n    restart: unless-stopped\n  nginx:\n    image: nginx:1.27\n"
	path := writeTemp(t, content)
	res, _ := SyncComposeImage(path, "komari", "ghcr.io/zhemed/komari:0.0.18")
	if res.Changed {
		t.Fatalf("目标 service 没有 image 行时不该改别的块")
	}
	if !strings.Contains(res.Reason, "没有 image 行") {
		t.Fatalf("原因不对：%q", res.Reason)
	}
}

func TestComposeInfoFromLabelsAndBinds(t *testing.T) {
	info := NewComposeInfo(map[string]string{
		composeServiceLabel: "komari",
		composeFilesLabel:   "/opt/docker/komari/compose.yaml, /opt/docker/komari/override.yml",
	})
	if !info.Usable() || info.Service != "komari" || len(info.Files) != 2 {
		t.Fatalf("解析 labels 失败：%+v", info)
	}
	binds := info.DirBinds()
	if len(binds) != 1 || binds[0] != "/opt/docker/komari:/opt/docker/komari" {
		t.Fatalf("同目录文件应去重成一个挂载：%v", binds)
	}
	// 非 compose 部署：不可用、不产生任何挂载
	empty := NewComposeInfo(map[string]string{})
	if empty.Usable() || len(empty.DirBinds()) != 0 {
		t.Fatalf("非 compose 部署不该产生挂载：%+v", empty)
	}
}
