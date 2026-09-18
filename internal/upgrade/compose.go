package upgrade

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/komari-monitor/komari/internal/dockerapi"
)

// docker compose 打在容器上的标签（compose v2 的稳定接口）。用它们就能知道
// "这个容器来自哪个 compose 文件、属于哪个 service"，从而在升级后把文件里的
// image tag 同步成新版本——否则文件的 tag 会落后，之后任何一次文件编辑都会
// 按旧 tag 把版本拉回去（见 docs/MAINTAINING.md §15.4）。
const (
	composeServiceLabel = "com.docker.compose.service"
	composeFilesLabel   = "com.docker.compose.project.config_files"
)

// ComposeInfo 是"本容器由 docker compose 管理"时我们需要的最小信息。
// 非 compose 部署（或标签缺失）时 Service/Files 为空，所有相关动作都会跳过。
type ComposeInfo struct {
	Service string
	Files   []string
}

// Usable 表示信息完整到足以做同步。
func (c ComposeInfo) Usable() bool {
	return c.Service != "" && len(c.Files) > 0
}

// NewComposeInfo 从容器 labels 解析 compose 信息；不是 compose 部署返回零值。
func NewComposeInfo(labels map[string]string) ComposeInfo {
	info := ComposeInfo{Service: strings.TrimSpace(labels[composeServiceLabel])}
	for _, f := range strings.Split(labels[composeFilesLabel], ",") {
		if f = strings.TrimSpace(f); f != "" {
			info.Files = append(info.Files, f)
		}
	}
	return info
}

// composeInfoFromContainer 从 inspect 结果里取 labels 解析。
func composeInfoFromContainer(self dockerapi.Container) ComposeInfo {
	labels := map[string]string{}
	if raw, ok := self.Config["Labels"].(map[string]any); ok {
		for k, v := range raw {
			if s, ok := v.(string); ok {
				labels[k] = s
			}
		}
	}
	return NewComposeInfo(labels)
}

// DirBinds 返回 helper 需要挂载的 compose 目录（`源:目标`，同路径）。
//
// 为什么挂**目录**而不是文件本身：同步用"写临时文件 + rename"做原子替换，
// 而 rename 到 bind 挂载的单个文件上会失败（EBUSY / 跨设备）；挂父目录才成立。
func (c ComposeInfo) DirBinds() []string {
	seen := map[string]bool{}
	var binds []string
	for _, f := range c.Files {
		dir := filepath.Dir(f)
		if dir == "" || dir == "/" || dir == "." || seen[dir] {
			continue
		}
		seen[dir] = true
		binds = append(binds, dir+":"+dir)
	}
	return binds
}

// ComposeSyncResult 描述一次 compose 文件同步的结果。
type ComposeSyncResult struct {
	File       string
	OldImage   string
	NewImage   string
	BackupPath string
	// Changed 为 true 表示文件确实被改写；false 时 Reason 说明为什么没改。
	Changed bool
	Reason  string
}

// syncServiceLine 匹配 service 名那一行（允许引号、允许行尾注释），捕获缩进。
var syncServiceLine = regexp.MustCompile(`^(\s*)"?([A-Za-z0-9._-]+)"?\s*:\s*(#.*)?$`)

// syncImageLine 匹配 service 块里的 image 行，捕获：缩进 / `image:` / 引号 / 值 / 引号 / 空格 / 注释。
var syncImageLine = regexp.MustCompile(`^(\s*)(image\s*:\s*)(["']?)([^"'\s#]+)(["']?)(\s*)(#.*)?$`)

// SyncComposeImage 把 file 里目标 service 的 image tag 换成 newImageRef 的 tag。
//
// 只换 tag、保留仓库名（兼容镜像加速与私有 registry，与 resolveTargetImage 同口径）；
// 认不出（image 用了变量、找不到 service 或 image 行）时返回 Changed=false + 原因，
// **不返回错误**——这类情况只应记日志，不该影响升级本身。
func SyncComposeImage(file, service, newImageRef string) (ComposeSyncResult, error) {
	res := ComposeSyncResult{File: file}
	if file == "" || service == "" {
		res.Reason = "缺少 compose 文件或 service 名"
		return res, nil
	}
	newTag := dockerapi.ParseImageRef(newImageRef).Tag
	if newTag == "" {
		res.Reason = "目标镜像引用解析不出 tag"
		return res, nil
	}

	info, err := os.Stat(file)
	if err != nil {
		res.Reason = "compose 文件不可读：" + err.Error()
		return res, nil
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		res.Reason = "compose 文件不可读：" + err.Error()
		return res, nil
	}

	lines := strings.Split(string(raw), "\n")
	start, serviceIndent := -1, ""
	for i, line := range lines {
		m := syncServiceLine.FindStringSubmatch(line)
		if m != nil && m[2] == service {
			start, serviceIndent = i, m[1]
			break
		}
	}
	if start < 0 {
		res.Reason = "文件里找不到 service " + service
		return res, nil
	}

	// service 块的结束：下一个非空、非注释、缩进不深于 service 行的行。
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if leadingSpaces(lines[i]) <= len(serviceIndent) {
			end = i
			break
		}
	}

	for i := start + 1; i < end; i++ {
		m := syncImageLine.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		oldValue := m[4]
		if strings.Contains(oldValue, "${") {
			res.Reason = "image 用了变量，跳过（不猜变量值）"
			return res, nil
		}
		res.OldImage = oldValue
		res.NewImage = dockerapi.ParseImageRef(oldValue).WithTag(newTag)
		if res.NewImage == "" {
			res.Reason = "镜像引用解析失败：" + oldValue
			return res, nil
		}
		if res.NewImage == oldValue {
			res.Reason = "tag 已经是目标版本"
			return res, nil
		}

		// 改写前留一份备份（覆盖上一份即可，它只服务于"这次写坏了"）。
		backup := file + ".bak"
		if err := os.WriteFile(backup, raw, info.Mode().Perm()); err != nil {
			res.Reason = "写备份失败：" + err.Error()
			return res, nil
		}
		res.BackupPath = backup

		lines[i] = m[1] + m[2] + m[3] + res.NewImage + m[5] + m[6] + m[7]
		tmp := file + ".komari-tmp"
		if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")), info.Mode().Perm()); err != nil {
			res.Reason = "写临时文件失败：" + err.Error()
			return res, nil
		}
		if err := os.Rename(tmp, file); err != nil {
			_ = os.Remove(tmp)
			res.Reason = "替换失败：" + err.Error()
			return res, nil
		}
		res.Changed = true
		return res, nil
	}

	res.Reason = "service 块里没有 image 行"
	return res, nil
}

// leadingSpaces 返回行首空格数（tab 按 1 计；compose 文件里缩进一致，够用）。
func leadingSpaces(line string) int {
	n := 0
	for _, r := range line {
		if r == ' ' || r == '\t' {
			n++
			continue
		}
		break
	}
	return n
}
