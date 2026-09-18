package dockerapi

import (
	"context"
	"net/url"
	"os"
	"strings"
)

// 本文件解决一个实测出来的缺口（2026-09-18）：
//
// `network_mode: host` 的容器里 **hostname 是宿主名**（不是容器 ID），而 cgroup v2 只有
// `0::/`——DetectSelfContainerID 的两条老线索全废，于是面板判不出"自己是谁"，
// 静默退回"容器内替换"（重建容器模式形同虚设）。
//
// 新线索：容器内部能读到 /proc/self/mountinfo 里 **bind 挂载的宿主路径**（root 字段），
// 而 Docker 的容器列表接口会返回每个容器的 Mounts（Source/Destination）。
// 两边比对即可唯一认出自己——与网络模式、hostname、cgroup 版本都无关。

// mountPair 是一条 bind 挂载：容器内挂载点 + 宿主机路径。
type mountPair struct {
	MountPoint string // 例如 /app/data
	HostRoot   string // 例如 /opt/docker/komari/data
}

// ContainerSummary 是 GET /containers/json 的条目（只取识别自身需要的字段）。
type ContainerSummary struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	Labels map[string]string `json:"Labels"`
	Mounts []Mount           `json:"Mounts"`
	State  string            `json:"State"`
}

// ListContainers 列出容器（含已停止的；识别自身时需要在全量里找）。
func (c *Client) ListContainers(ctx context.Context) ([]ContainerSummary, error) {
	var items []ContainerSummary
	// 注意：do() 内部会自己拼 /v<api> 前缀，这里只能传"路径 + 查询串"，不能传整条 URL
	//（否则拼成 http://docker/v1.41http://docker/...，实测表现为 404 page not found）。
	q := url.Values{}
	q.Set("all", "1")
	if err := c.do(ctx, "GET", "/containers/json?"+q.Encode(), nil, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// SelfContainerID 识别"本进程所在容器"的 ID。
//
// 顺序：hostname 短 ID → cgroup 长 ID →（新增）bind 挂载比对。
// 前两条来自上游写法，只在"非 host 网络 + cgroup v1/带容器路径"时成立；
// 第三条是 host 网络下的唯一可靠线索。
func (c *Client) SelfContainerID(ctx context.Context) (string, error) {
	if id := DetectSelfContainerID(); id != "" {
		return id, nil
	}
	mounts := selfBindMounts(readSelfMountInfo())
	if len(mounts) == 0 {
		return "", errSelfUnidentified
	}
	items, err := c.ListContainers(ctx)
	if err != nil {
		return "", err
	}
	if id := matchSelfContainer(mounts, items); id != "" {
		return id, nil
	}
	return "", errSelfUnidentified
}

var errSelfUnidentified = errSelf("无法识别自身容器（hostname/cgroup 无线索，bind 挂载也匹配不上）")

type errSelf string

func (e errSelf) Error() string { return string(e) }

// selfMountInfoPath 是读取自身挂载表的路径；测试里可指向临时文件。
var selfMountInfoPath = "/proc/self/mountinfo"

func readSelfMountInfo() string {
	data, err := os.ReadFile(selfMountInfoPath)
	if err != nil {
		return ""
	}
	return string(data)
}

// parseBindMounts 从 mountinfo 文本里解析出 bind 挂载对。
//
// mountinfo 每行格式（空格分隔）：
//
//	mountID parentID major:minor root mountPoint options [optional...] - fstype source superOptions
//
// bind 挂载的特征：**root 不是 `/`**，而是宿主上的绝对路径。容器自己那层 overlay 的
// root 是 `/`，所以天然被排除。
func parseBindMounts(mountinfo string) []mountPair {
	var out []mountPair
	for _, line := range strings.Split(mountinfo, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}
		sep := -1
		for i, f := range fields {
			if f == "-" {
				sep = i
				break
			}
		}
		if sep < 0 || sep+2 >= len(fields) {
			continue
		}
		root, mountPoint := fields[3], fields[4]
		if root == "/" || !strings.HasPrefix(root, "/") {
			continue
		}
		out = append(out, mountPair{MountPoint: strings.TrimSuffix(mountPoint, "/"), HostRoot: strings.TrimSuffix(root, "/")})
	}
	return out
}

// selfBindMounts 只保留"看起来像我们自己的" bind 挂载：挂载点在容器内的绝对路径。
func selfBindMounts(mountinfo string) []mountPair {
	pairs := parseBindMounts(mountinfo)
	kept := pairs[:0]
	for _, p := range pairs {
		if p.MountPoint == "" || p.HostRoot == "" || p.MountPoint == p.HostRoot {
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

// matchSelfContainer 在容器列表里找与本进程挂载对得上的容器。
// 返回匹配的容器 ID；没有匹配返回空串。匹配得分 = 完全一致的 (Source, Destination) 对数。
func matchSelfContainer(self []mountPair, items []ContainerSummary) string {
	best, bestScore := "", 0
	for _, item := range items {
		score := 0
		for _, p := range self {
			for _, m := range item.Mounts {
				if m.Type != "bind" {
					continue
				}
				if strings.TrimSuffix(m.Source, "/") == p.HostRoot &&
					strings.TrimSuffix(m.Destination, "/") == p.MountPoint {
					score++
					break
				}
			}
		}
		if score > bestScore {
			best, bestScore = item.ID, score
		}
	}
	return best
}
