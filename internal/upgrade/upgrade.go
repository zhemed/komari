package upgrade

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ErrUpToDate 表示当前已经是最新稳定版（未指定 tag 时的正常结果，不是故障）。
var ErrUpToDate = errors.New("already running the latest stable version")

// DefaultImage 是容器形态提示里使用的镜像名。
const DefaultImage = "ghcr.io/zhemed/komari"

// Options 是一次升级所需的全部外部输入。
type Options struct {
	Repo       string // owner/repo，来自服务端设置
	CurrentTag string // 当前运行版本
	BinaryPath string // 当前二进制路径（空则用 os.Executable）
	// StateDir 是状态文件与"仅下载"模式的目标目录。
	//
	// 偏离设计说明（2026-09-17）：原设计写"data 目录"，但仓库里没有能从 web 层安全使用的
	// 数据目录 helper（DB 路径在 cmd 包里，web→cmd 会形成导入环）。改用二进制同目录：
	// 升级本就要写这个目录（临时文件 + 原子替换），前置检查也已确认它可写。
	StateDir      string
	Image         string // 容器提示用镜像名，空则 DefaultImage
	HTTP          *http.Client
	APIBase       string
	Probe         VersionProbe // 自检探针，测试可注入
	Env           *Environment // 部署形态，测试可注入；nil 时自动探测
	VersionLister *Client      // 测试可注入；nil 时按 Repo/HTTP/APIBase 构造
}

func (o Options) client() *Client {
	if o.VersionLister != nil {
		return o.VersionLister
	}
	return &Client{Repo: o.Repo, HTTP: o.HTTP, APIBase: o.APIBase}
}

func (o Options) environment() (Environment, error) {
	if o.Env != nil {
		return *o.Env, nil
	}
	return DetectEnvironment(o.BinaryPath)
}

// updatesDir 返回"仅下载"模式的落盘目录：优先二进制同目录，其次系统临时目录。
func (o Options) updatesDir(env Environment) string {
	base := o.StateDir
	if base == "" {
		base = filepath.Dir(env.BinaryPath)
	}
	dir := filepath.Join(base, "upgrades")
	if dirWritable(base) {
		return dir
	}
	return filepath.Join(os.TempDir(), "komari-upgrades")
}

func (o Options) image() string {
	if strings.TrimSpace(o.Image) != "" {
		return strings.TrimSpace(o.Image)
	}
	return DefaultImage
}

// Plan 是"要做什么"的描述：由 Prepare 产出，可被 RPC 直接回给前端。
type Plan struct {
	From         string `json:"from"`
	To           string `json:"to"`
	AssetName    string `json:"asset_name,omitempty"`
	Manual       bool   `json:"manual"` // 容器：必须手工拉镜像，服务端不做任何替换
	PullCommand  string `json:"pull_command,omitempty"`
	DownloadOnly bool   `json:"download_only"` // 无服务管理器：只下载，不替换不退出
	DownloadPath string `json:"download_path,omitempty"`
	assetURL     string
	checksum     string
}

// Result 是执行结果。
type Result struct {
	From         string `json:"from"`
	To           string `json:"to"`
	BackupPath   string `json:"backup_path,omitempty"`
	DownloadOnly bool   `json:"download_only,omitempty"`
	DownloadPath string `json:"download_path,omitempty"`
}

// Prepare 解析目标版本、做前置检查、取校验和，产出可执行计划。
//
// tag 为空表示"升到最新稳定版"；非空表示安装指定版本（回滚也走这条路）。
func Prepare(ctx context.Context, o Options, tag string) (Plan, error) {
	current := strings.TrimSpace(o.CurrentTag)
	tag = strings.TrimSpace(tag)
	client := o.client()

	var target Release
	if tag == "" {
		latest, newer, err := client.LatestStable(ctx, current)
		if err != nil {
			return Plan{}, err
		}
		if !newer {
			return Plan{}, ErrUpToDate
		}
		target = latest
	} else {
		var err error
		target, err = client.FindRelease(ctx, tag)
		if err != nil {
			return Plan{}, err
		}
		if target.Tag == current {
			return Plan{}, fmt.Errorf("目标版本 %s 就是当前运行版本", target.Tag)
		}
	}

	plan := Plan{From: current, To: target.Tag}

	env, err := o.environment()
	if err != nil {
		return Plan{}, err
	}
	if env.Container {
		// 容器里替换二进制会随容器重建丢失：只给可复制的拉取命令。
		plan.Manual = true
		plan.PullCommand = PullHint(o.image(), target.Tag)
		return plan, nil
	}
	assetName, ok := ServerAssetName(runtime.GOOS, runtime.GOARCH)
	if !ok {
		return Plan{}, fmt.Errorf("当前平台 %s/%s 没有发布资产，请手工升级",
			runtime.GOOS, runtime.GOARCH)
	}
	asset, ok := target.Asset(assetName)
	if !ok {
		return Plan{}, fmt.Errorf("release %s 里没有资产 %s", target.Tag, assetName)
	}
	sums, err := client.FetchChecksums(ctx, target)
	if err != nil {
		return Plan{}, err
	}
	sum, ok := sums[assetName]
	if !ok {
		return Plan{}, fmt.Errorf("%s 里没有 %s 的校验和", ChecksumAssetName, assetName)
	}
	if sum == "" {
		return Plan{}, fmt.Errorf("%s 里 %s 的校验和为空", ChecksumAssetName, assetName)
	}

	plan.AssetName = assetName
	plan.assetURL = asset.URL
	plan.checksum = sum

	if !env.Systemd {
		// 没有服务管理器接手：只下载 + 给命令，绝不自杀式退出。
		plan.DownloadOnly = true
		plan.DownloadPath = filepath.Join(o.updatesDir(env), assetName)
		return plan, nil
	}
	if !env.DirWritable {
		return Plan{}, fmt.Errorf("%w: %s", ErrDirNotWritable, filepath.Dir(env.BinaryPath))
	}
	return plan, nil
}

// Execute 执行计划：下载 → 校验 → 自检 → （非仅下载时）原子替换。
// 成功返回 Result；调用方负责在需要时重启进程（见 RPC 层）。
func Execute(ctx context.Context, o Options, p Plan) (Result, error) {
	if p.Manual {
		return Result{From: p.From, To: p.To, DownloadOnly: true}, nil
	}
	if p.assetURL == "" {
		return Result{}, fmt.Errorf("计划缺少下载地址（Prepare 未成功执行）")
	}

	env, err := o.environment()
	if err != nil {
		return Result{}, err
	}
	if p.DownloadOnly && p.DownloadPath == "" {
		p.DownloadPath = filepath.Join(o.StateDir, "upgrades", filepath.Base(p.assetURL))
	}
	dst := p.DownloadPath
	if !p.DownloadOnly {
		dst = filepath.Join(filepath.Dir(env.BinaryPath), ".komari-upgrade-"+p.To+".tmp")
	}

	SetStatus(Status{Phase: PhaseDownloading, From: p.From, To: p.To, Running: true})
	if p.DownloadOnly {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return Result{}, err
		}
		_ = SaveState(o.StateDir, Status{Phase: PhaseDownloading, From: p.From, To: p.To})
	}
	if err := o.client().DownloadVerified(ctx, p.assetURL, dst, p.checksum); err != nil {
		SetStatus(Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		_ = SaveState(o.StateDir, Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		return Result{}, err
	}

	SetStatus(Status{Phase: PhaseVerifying, From: p.From, To: p.To, Running: true})
	// 自检要**执行**这个文件：先补上执行位。
	// 2026-09-17 实测踩到：下载出来是 0644，直接 fork/exec 会 "permission denied"，
	// 表现为"升级失败但服务不受影响"（替换发生在自检之后，所以不会伤到线上）。
	if err := os.Chmod(dst, 0o755); err != nil {
		_ = os.Remove(dst)
		SetStatus(Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		_ = SaveState(o.StateDir, Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		return Result{}, err
	}
	if err := VerifyBinary(ctx, o.Probe, dst, p.To); err != nil {
		_ = os.Remove(dst)
		SetStatus(Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		_ = SaveState(o.StateDir, Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		return Result{}, err
	}

	if p.DownloadOnly {
		res := Result{From: p.From, To: p.To, DownloadOnly: true, DownloadPath: dst}
		_ = SaveState(o.StateDir, Status{Phase: PhaseCompleted, From: p.From, To: p.To, UpdatesDir: filepath.Dir(dst)})
		SetStatus(Status{Phase: PhaseCompleted, From: p.From, To: p.To, UpdatesDir: filepath.Dir(dst)})
		return res, nil
	}

	SetStatus(Status{Phase: PhaseReplacing, From: p.From, To: p.To, Running: true})
	backup, err := SwapBinary(env.BinaryPath, dst, p.From)
	if err != nil {
		SetStatus(Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		_ = SaveState(o.StateDir, Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		return Result{}, err
	}

	res := Result{From: p.From, To: p.To, BackupPath: backup}
	SetStatus(Status{Phase: PhaseRestarting, From: p.From, To: p.To, BackupPath: backup, Running: true})
	_ = SaveState(o.StateDir, Status{Phase: PhaseRestarting, From: p.From, To: p.To, BackupPath: backup})
	return res, nil
}

// Reconcile 把落盘状态与"当前实际运行的版本"对齐：
// 若上次是 restarting 且进程已换成目标版本，则标记为 completed 并落盘。
func Reconcile(stateDir, currentVersion string) Status {
	st, err := LoadState(stateDir)
	if err != nil {
		return Status{Phase: PhaseFailed, Error: err.Error(), UpdatedAt: time.Now().UTC()}
	}
	if st.Phase == PhaseRestarting && strings.TrimSpace(currentVersion) == strings.TrimSpace(st.To) {
		st.Phase = PhaseCompleted
		st.Error = ""
		_ = SaveState(stateDir, st)
	}
	return st
}
