package dockerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// DefaultBinaryPath 是镜像里 komari 二进制的位置（见 Dockerfile：COPY … /app/komari）。
const DefaultBinaryPath = "/app/komari"

var (
	reShortID = regexp.MustCompile(`^[0-9a-f]{12}$`)
	reLongID  = regexp.MustCompile(`[0-9a-f]{64}`)
)

// DetectSelfContainerID 推断"自己所在的容器"。
//
// 依据（按可靠性排序）：
//  1. hostname：Docker 默认把容器 hostname 设为容器短 ID —— 也是最常见的形态；
//  2. /proc/self/cgroup：cgroup v1 的路径里含 64 位容器 ID（cgroup v2 常常不含）。
//
// 两条都拿不到就返回空串，调用方应回落到"手工模式"，不要猜。
func DetectSelfContainerID() string {
	if host, err := os.Hostname(); err == nil {
		host = strings.TrimSpace(host)
		if reShortID.MatchString(host) {
			return host
		}
	}
	if data, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		if id := reLongID.FindString(string(data)); id != "" {
			return id
		}
	}
	return ""
}

// BuildRecreatePayload 用旧容器的 inspect 结果拼出"建新容器"的请求体。
//
// 原则：**逐字段沿用**旧的 Config 与 HostConfig，只替换镜像；网络别名/静态 IP 也照抄。
// 唯一的例外是 Hostname：Docker 未显式指定时会把 Config.Hostname 填成容器 ID，
// 照抄会把这个旧 ID 钉在新容器上（而我们的"自身容器识别"依赖 hostname==短 ID），所以
// 当 Hostname 等于旧容器短 ID 时删掉它，交给 daemon 重新分配。
func BuildRecreatePayload(old Container, newImage, newName string) (map[string]any, error) {
	if strings.TrimSpace(newImage) == "" {
		return nil, fmt.Errorf("目标镜像为空")
	}
	if old.Config == nil {
		return nil, fmt.Errorf("旧容器缺少 Config（inspect 结果不完整）")
	}
	payload := make(map[string]any, len(old.Config)+2)
	for k, v := range old.Config {
		payload[k] = v
	}
	payload["Image"] = newImage

	if host, ok := payload["Hostname"].(string); ok {
		short := strings.TrimPrefix(old.ID, "/")
		if len(short) > 12 {
			short = short[:12]
		}
		if host == short || host == strings.TrimPrefix(old.ID, "/") {
			delete(payload, "Hostname")
		}
	}

	if old.HostConfig != nil {
		payload["HostConfig"] = old.HostConfig
	}
	if eps := buildEndpointsConfig(old.NetworkSettings.Networks); len(eps) > 0 {
		payload["NetworkingConfig"] = map[string]any{"EndpointsConfig": eps}
	}
	_ = newName // 容器名通过 create 的 query 参数传，不放在 body 里
	return payload, nil
}

// buildEndpointsConfig 从 inspect 的 NetworkSettings.Networks 还原创建时需要的那几个字段。
func buildEndpointsConfig(networks map[string]any) map[string]any {
	out := map[string]any{}
	for name, raw := range networks {
		cfg, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ep := map[string]any{}
		for _, key := range []string{"Aliases", "IPAMConfig", "Links", "DriverOpts"} {
			if v, ok := cfg[key]; ok && v != nil {
				ep[key] = v
			}
		}
		out[name] = ep
	}
	return out
}

// RecreateResult 描述重建结果。
type RecreateResult struct {
	ContainerName    string // 原名（新容器沿用）
	NewContainerID   string
	OldContainerName string // 旧容器改名后的名字（保留为回滚点）
	Noop             bool   // 镜像本来就一样
}

// RecreateOptions 是 helper 的全部输入。
type RecreateOptions struct {
	Container  string // 旧容器 id 或名字
	NewImage   string // 目标镜像（repo:tag）
	SanityTag  string // 预检时要求出现在 --help 输出里的版本号
	BinaryPath string // 镜像内二进制路径，空则 DefaultBinaryPath
	Progress   func(phase, detail string)
}

// Recreate 执行"把容器换成新镜像"的全流程，任何一步失败都尽量回滚到旧容器可用。
//
// 注意执行顺序：先 create 新容器（不冲突），再 stop 旧容器（释放端口、同时会终止旧进程），
// 最后 start 新容器；失败则 remove 新容器 + 把旧容器改名回去并启动。
func (c *Client) Recreate(ctx context.Context, opts RecreateOptions) (RecreateResult, error) {
	progress := func(phase, detail string) {
		if opts.Progress != nil {
			opts.Progress(phase, detail)
		}
	}
	binaryPath := opts.BinaryPath
	if binaryPath == "" {
		binaryPath = DefaultBinaryPath
	}
	var res RecreateResult

	old, err := c.InspectContainer(ctx, opts.Container)
	if err != nil {
		return res, fmt.Errorf("读取自身容器信息失败：%w", err)
	}
	name := strings.TrimPrefix(old.Name, "/")
	res.ContainerName = name
	if oldImage, _ := old.Config["Image"].(string); oldImage == opts.NewImage {
		res.Noop = true
		return res, nil
	}

	// 1) 预检：用新镜像跑 --help，确认版本行正确（不通过就什么都不动）。
	progress("precheck", "precheck new image")
	code, logs, err := c.RunOnce(ctx, opts.NewImage, []string{binaryPath, "--help"}, "", "")
	if err != nil {
		return res, fmt.Errorf("预检容器启动失败：%w", err)
	}
	if want := "Komari Monitor " + strings.TrimSpace(opts.SanityTag); opts.SanityTag != "" && !strings.Contains(logs, want) {
		return res, fmt.Errorf("预检失败：新镜像输出里没有 %q（退出码 %d）", want, code)
	}

	// 2) 旧容器改名，腾出原名
	oldName := fmt.Sprintf("%s-old-%s", name, time.Now().UTC().Format("20060102-150405"))
	if err := c.RenameContainer(ctx, old.ID, oldName); err != nil {
		return res, fmt.Errorf("旧容器改名失败：%w", err)
	}
	res.OldContainerName = oldName

	rollback := func(cause error) (RecreateResult, error) {
		progress("rollback", cause.Error())
		rbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
		defer cancel()
		if err := c.RenameContainer(rbCtx, old.ID, name); err != nil {
			return res, fmt.Errorf("%v；且回滚改名失败：%v", cause, err)
		}
		if err := c.StartContainer(rbCtx, old.ID); err != nil {
			return res, fmt.Errorf("%v；且回滚启动失败：%v", cause, err)
		}
		return res, cause
	}

	// 3) 建新容器（沿用旧配置 + 新镜像）
	payload, err := BuildRecreatePayload(old, opts.NewImage, name)
	if err != nil {
		return rollback(err)
	}
	progress("creating", "create replacement container")
	newID, err := c.CreateContainer(ctx, name, payload)
	if err != nil {
		return rollback(fmt.Errorf("创建新容器失败：%w", err))
	}
	res.NewContainerID = newID

	// 4) 停旧容器（释放端口；这也会终止旧进程本身）
	progress("stopping", "stop old container")
	if err := c.StopContainer(ctx, old.ID, 15); err != nil {
		_ = c.RemoveContainer(context.WithoutCancel(ctx), newID, true)
		return rollback(fmt.Errorf("停止旧容器失败：%w", err))
	}

	// 5) 起新容器
	progress("starting", "start replacement container")
	if err := c.StartContainer(ctx, newID); err != nil {
		_ = c.RemoveContainer(context.WithoutCancel(ctx), newID, true)
		return rollback(fmt.Errorf("启动新容器失败：%w", err))
	}
	progress("done", "replacement started")
	return res, nil
}

// MarshalPayload 便于把 payload 落进日志/状态（排障用，剔除环境变量里的敏感值不做处理——
// 调用方若要记录请注意脱敏）。
func MarshalPayload(payload map[string]any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}
