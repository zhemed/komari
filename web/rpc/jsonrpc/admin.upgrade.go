package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/internal/upgrade"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/utils"
	logger "github.com/komari-monitor/komari/utils/log"
)

// admin.upgrade.go
// 面板一键升级（server self-upgrade）：只做"服务端替换自己的二进制"这一件事，
// 目标是 systemd + 二进制安装的形态。
//
// 安全边界（v1，明确记录）：
//   - 目标仓库来自服务端设置，**不接受请求参数里的 URL**；
//   - 需要管理员权限（admin 命名空间）并写审计日志；
//   - 校验 SHA256 只能保证"下载内容与发布清单一致"，不防发布链被篡改（签名体系留 v2）；
//   - 故意**没有**标记 rpc.MarkSensitive：那会要求每次调用都带 2FA 码，
//     而面板目前没有该提示流程。若后续接上 2FA 提示，这里应改为 MarkSensitive("admin:upgradeServer")。

const defaultUpgradeRepo = "zhemed/komari"

// serverUpgradeSettings 是持久化的开关与目标仓库（两个扁平配置键）。
type serverUpgradeSettings struct {
	Repo    string `json:"repo"`
	Enabled bool   `json:"enabled"`
}

// loadServerUpgradeSettings 读取设置并补齐缺省值（配置缺失时不应让功能不可用）。
func loadServerUpgradeSettings() serverUpgradeSettings {
	repo, err := config.GetAs[string](config.ServerUpdateRepoKey, defaultUpgradeRepo)
	if err != nil || strings.TrimSpace(repo) == "" {
		repo = defaultUpgradeRepo
	}
	enabled, err := config.GetAs[bool](config.ServerUpgradeEnabledKey, true)
	if err != nil {
		enabled = true
	}
	return serverUpgradeSettings{Repo: repo, Enabled: enabled}
}

// validateUpgradeSettingChanges 供通用设置接口（admin:editSettings）在落库前调用，
// 拦住非法仓库名——写进配置后被升级流程读到才报错就太晚了。
func validateUpgradeSettingChanges(cfg map[string]interface{}) error {
	if v, ok := cfg[config.ServerUpdateRepoKey]; ok {
		repo, isStr := v.(string)
		if !isStr {
			return fmt.Errorf("%s 必须是字符串", config.ServerUpdateRepoKey)
		}
		if err := validateUpgradeRepo(strings.TrimSpace(repo)); err != nil {
			return err
		}
	}
	return nil
}

// upgradeExit 是"替换完成后退出，交给 systemd 拉起"的出口；测试可替换。
var upgradeExit = func(code int) {
	// 给 RPC 响应和日志一点落盘时间，避免还没回包就退出。
	time.Sleep(500 * time.Millisecond)
	os.Exit(code)
}

var upgradeStartMu sync.Mutex

func init() {
	reg("getServerUpgradeSettings", adminGetServerUpgradeSettings, "Get server self-upgrade settings")
	reg("listServerReleases", adminListServerReleases, "List server releases available for install")
	reg("upgradeServer", adminUpgradeServer, "Download and install a server release (self-upgrade)")
	reg("upgradeStatus", adminUpgradeStatus, "Get server self-upgrade status")
}

func adminGetServerUpgradeSettings(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	s := loadServerUpgradeSettings()
	return map[string]any{
		"repo":    s.Repo,
		"enabled": s.Enabled,
		// 当前形态是否真能一键升级：容器/无 systemd/非 linux 都会在这里体现，供前端决定按钮文案。
		"supported": upgradeSupported(),
		"platform":  upgradePlatform(),
	}, nil
}

func adminListServerReleases(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		Limit int `json:"limit"`
	}
	_ = req.BindParams(&params)
	if params.Limit <= 0 || params.Limit > 50 {
		params.Limit = 20
	}
	settings := loadServerUpgradeSettings()
	client := &upgrade.Client{Repo: settings.Repo}
	list, err := client.StableReleases(ctx)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "查询 release 失败: "+err.Error(), nil)
	}
	out := make([]map[string]any, 0, params.Limit)
	for i, r := range list {
		if i >= params.Limit {
			break
		}
		out = append(out, map[string]any{
			"tag":          r.Tag,
			"name":         r.Name,
			"published_at": r.PublishedAt,
			"current":      r.Tag == utils.CurrentVersion,
		})
	}
	return map[string]any{
		"releases":         out,
		"current_version":  utils.CurrentVersion,
		"repo":             settings.Repo,
		"checksum_asset":   upgrade.ChecksumAssetName,
		"upgrade_supports": upgradeSupported(),
	}, nil
}

func adminUpgradeServer(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var params struct {
		Tag string `json:"tag"`
	}
	_ = req.BindParams(&params)

	settings := loadServerUpgradeSettings()
	if !settings.Enabled {
		return nil, rpc.MakeError(rpc.PermissionDenied, "一键升级已在设置中关闭", nil)
	}

	upgradeStartMu.Lock()
	if upgrade.IsRunning() {
		upgradeStartMu.Unlock()
		return nil, rpc.MakeError(rpc.InvalidParams, "已有升级任务在进行中", nil)
	}
	upgrade.MarkRunning(true)
	upgradeStartMu.Unlock()

	opts, err := buildUpgradeOptions(settings)
	if err != nil {
		upgrade.MarkRunning(false)
		return nil, rpc.MakeError(rpc.InternalError, err.Error(), nil)
	}

	plan, err := upgrade.Prepare(ctx, opts, params.Tag)
	if err != nil {
		upgrade.MarkRunning(false)
		switch {
		case errors.Is(err, upgrade.ErrUpToDate):
			return nil, rpc.MakeError(rpc.InvalidParams, "当前已经是最新稳定版", nil)
		default:
			return nil, rpc.MakeError(rpc.InvalidParams, err.Error(), nil)
		}
	}

	actor, ip := auditActor(ctx)
	auditlog.Log(ip, actor, fmt.Sprintf("server upgrade requested: %s -> %s", plan.From, plan.To), "info")

	// 容器形态：不替换、不退出，直接把可复制的命令回给面板。
	if plan.Manual {
		upgrade.MarkRunning(false)
		return map[string]any{
			"started":      false,
			"manual":       true,
			"to":           plan.To,
			"from":         plan.From,
			"pull_command": plan.PullCommand,
			"message":      "容器内不能替换二进制，请拉取新镜像后重建容器",
		}, nil
	}

	go runServerUpgrade(opts, plan)
	return map[string]any{
		"started":       true,
		"from":          plan.From,
		"to":            plan.To,
		"download_only": plan.DownloadOnly,
	}, nil
}

// runServerUpgrade 在后台执行升级：下载 → 校验 → 自检 → 替换 → 退出（交 systemd 拉起）。
func runServerUpgrade(opts upgrade.Options, plan upgrade.Plan) {
	defer upgrade.MarkRunning(false)
	res, err := upgrade.Execute(context.Background(), opts, plan)
	if err != nil {
		logger.Errorf("upgrade", "server upgrade failed (%s -> %s): %v", plan.From, plan.To, err)
		return
	}
	if res.DownloadOnly {
		logger.Infof("upgrade", "已下载 %s 到 %s（当前进程没有 systemd 接管，未替换二进制）", res.To, res.DownloadPath)
		return
	}
	logger.Infof("upgrade", "已替换为 %s（备份 %s），退出等待 systemd 拉起", res.To, res.BackupPath)
	upgradeExit(42)
}

func adminUpgradeStatus(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	settings := loadServerUpgradeSettings()
	st := upgrade.StatusSnapshot()
	if st.Phase == upgrade.PhaseIdle {
		// 内存态为空（例如刚重启）：读落盘状态，并把 restarting→completed 收敛。
		st = upgrade.Reconcile(upgradeStateDir(), utils.CurrentVersion)
	}
	return map[string]any{
		"phase":           st.Phase,
		"from":            st.From,
		"to":              st.To,
		"error":           st.Error,
		"backup_path":     st.BackupPath,
		"updates_dir":     st.UpdatesDir,
		"running":         st.Running || upgrade.IsRunning(),
		"updated_at":      st.UpdatedAt,
		"current_version": utils.CurrentVersion,
		"enabled":         settings.Enabled,
		"repo":            settings.Repo,
		"supported":       upgradeSupported(),
		"platform":        upgradePlatform(),
	}, nil
}

// validateUpgradeRepo 校验 owner/repo 形式，避免把 URL 或注入内容写进配置。
func validateUpgradeRepo(repo string) error {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("repo 必须形如 owner/repo")
	}
	if strings.ContainsAny(repo, " \t\n\r?#&=") || strings.Contains(repo, "..") {
		return fmt.Errorf("repo 含非法字符")
	}
	return nil
}

// buildUpgradeOptions 组装升级参数：二进制路径取自 os.Executable，状态目录用二进制同目录。
func buildUpgradeOptions(settings serverUpgradeSettings) (upgrade.Options, error) {
	exe, err := os.Executable()
	if err != nil {
		return upgrade.Options{}, fmt.Errorf("无法定位当前二进制: %w", err)
	}
	return upgrade.Options{
		Repo:       settings.Repo,
		CurrentTag: utils.CurrentVersion,
		BinaryPath: exe,
		StateDir:   filepath.Dir(exe),
	}, nil
}

func upgradeStateDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

// upgradeSupported 返回当前形态能否做一键替换（容器/无 systemd 都不行）。
func upgradeSupported() bool {
	env, err := upgrade.DetectEnvironment("")
	if err != nil {
		return false
	}
	if env.Container || !env.Systemd || !env.DirWritable {
		return false
	}
	_, ok := upgrade.ServerAssetName(upgradePlatformPair())
	return ok
}

func upgradePlatformPair() (string, string) { return runtime.GOOS, runtime.GOARCH }

func upgradePlatform() string {
	goos, goarch := upgradePlatformPair()
	return goos + "/" + goarch
}
