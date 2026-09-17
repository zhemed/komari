package upgrade

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/komari-monitor/komari/internal/dockerapi"
)

const (
	// DefaultDockerSocket 是容器里 docker socket 的默认路径。
	DefaultDockerSocket = "/var/run/docker.sock"
	// DefaultImageBinaryPath 是镜像里 komari 二进制的位置（见 Dockerfile）。
	DefaultImageBinaryPath = "/app/komari"
)

// dockerSocketReady 判断 socket 是否真的可用（存在 + daemon 应答）。
// 只有真的可用才启用"重建容器"模式；否则一律回落到手工模式。
func dockerSocketReady(ctx context.Context, socketPath string) (selfID string, err error) {
	client := dockerapi.NewClient(socketPath)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx); err != nil {
		return "", err
	}
	if _, err := client.ServerVersion(pingCtx); err != nil {
		return "", err
	}
	selfID = dockerapi.DetectSelfContainerID()
	if selfID == "" {
		return "", fmt.Errorf("无法识别自身容器（hostname 与 /proc/self/cgroup 都不含容器 ID）")
	}
	return selfID, nil
}

// helperPayload 组装 helper 容器的创建请求。
//
// helper 用**目标镜像**启动（它必然带 docker-self-recreate 子命令），只挂 docker socket
// 与数据目录（写状态文件用），不挂业务需要的其它卷；跑完由 daemon 自动删除。
func helperPayload(helperImage, selfID, targetImage, sanityTag, stateDir, socketPath, binaryPath string, binds []string) map[string]any {
	if binaryPath == "" {
		binaryPath = DefaultImageBinaryPath
	}
	cmd := []string{
		binaryPath, "docker-self-recreate",
		"--container", selfID,
		"--image", targetImage,
		"--socket", socketPath,
		"--timeout", "10m",
	}
	if sanityTag != "" {
		cmd = append(cmd, "--sanity-tag", sanityTag)
	}
	if stateDir != "" {
		cmd = append(cmd, "--state-dir", stateDir)
	}
	return map[string]any{
		"Image": helperImage,
		"Cmd":   cmd,
		"HostConfig": map[string]any{
			"AutoRemove":    true,
			"NetworkMode":   "none",
			"RestartPolicy": map[string]any{"Name": "no"},
			"Binds":         binds,
		},
	}
}

// helperBinds 计算 helper 需要的卷：docker socket + 状态文件所在的数据目录。
// 用**被升级容器自己的 Mounts** 里的宿主路径，这样即使宿主路径与容器内路径不同也能挂对。
func helperBinds(self dockerapi.Container, socketPath, stateDir string) []string {
	var binds []string
	socketHost := ""
	for _, m := range self.Mounts {
		if m.Destination == socketPath || strings.HasSuffix(m.Destination, "docker.sock") {
			socketHost = m.Source
			break
		}
	}
	if socketHost == "" {
		// 没在 Mounts 里找到就按同路径挂（标准用法 -v /var/run/docker.sock:/var/run/docker.sock）
		socketHost = socketPath
	}
	binds = append(binds, socketHost+":"+socketPath)

	if stateDir != "" {
		for _, m := range self.Mounts {
			if m.Destination != "" && (stateDir == m.Destination || strings.HasPrefix(stateDir, strings.TrimSuffix(m.Destination, "/")+"/")) {
				binds = append(binds, m.Source+":"+m.Destination)
				break
			}
		}
	}
	return binds
}

// resolveTargetImage 由"自身容器的镜像名"推出目标镜像：保留 registry 与命名空间，只换 tag。
// 这样用镜像加速/私有 registry 的用户也能拉对；读不到就退回默认镜像名。
func (o Options) resolveTargetImage(ctx context.Context, tag string) string {
	if ref := pullHintImageFromSelf(ctx, o.socketPath(), o.SelfContainerID); ref != "" {
		return dockerapi.ParseImageRef(ref).WithTag(tag)
	}
	return dockerapi.ParseImageRef(o.image()).WithTag(tag)
}

// pullHintImageFromSelf 读取自身容器的镜像引用；失败返回空串。
func pullHintImageFromSelf(ctx context.Context, socketPath, selfID string) string {
	if selfID == "" {
		return ""
	}
	client := dockerapi.NewClient(socketPath)
	inspectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	self, err := client.InspectContainer(inspectCtx, selfID)
	if err != nil {
		return ""
	}
	if img, ok := self.Config["Image"].(string); ok {
		return img
	}
	return ""
}

// executeDockerRecreate 执行容器形态的升级：拉镜像 → 起 helper → 写状态 → 等 helper 停掉本容器。
func (o Options) executeDockerRecreate(ctx context.Context, p Plan) (Result, error) {
	socketPath := o.socketPath()
	client := dockerapi.NewClient(socketPath)
	selfID := o.SelfContainerID
	if selfID == "" {
		var err error
		selfID, err = dockerSocketReady(ctx, socketPath)
		if err != nil {
			return Result{}, err
		}
	}

	progress := func(detail string) {
		SetStatus(Status{Phase: PhaseDownloading, From: p.From, To: p.To, Detail: detail, Running: true})
	}
	progress("pulling " + p.TargetImage)
	if err := SaveState(o.StateDir, Status{Phase: PhaseDownloading, From: p.From, To: p.To, Image: p.TargetImage}); err != nil {
		return Result{}, err
	}
	if err := client.PullImage(ctx, p.TargetImage, progress); err != nil {
		SetStatus(Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		_ = SaveState(o.StateDir, Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		return Result{}, err
	}

	self, err := client.InspectContainer(ctx, selfID)
	if err != nil {
		return Result{}, fmt.Errorf("读取自身容器信息失败：%w", err)
	}
	binds := helperBinds(self, socketPath, o.StateDir)
	payload := helperPayload(p.TargetImage, selfID, p.TargetImage, p.To, o.StateDir, socketPath,
		o.ImageBinaryPath, binds)

	SetStatus(Status{Phase: PhaseReplacing, From: p.From, To: p.To, Detail: "recreating container", Running: true})
	helperID, err := client.CreateContainer(ctx, "", payload)
	if err != nil {
		SetStatus(Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		_ = SaveState(o.StateDir, Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		return Result{}, fmt.Errorf("启动 helper 容器失败：%w", err)
	}
	if err := client.StartContainer(ctx, helperID); err != nil {
		SetStatus(Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		_ = SaveState(o.StateDir, Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: err.Error()})
		return Result{}, fmt.Errorf("启动 helper 容器失败：%w", err)
	}

	// helper 会先建好同名新容器、再停掉本容器；这里只需停留到被停掉为止。
	// 若 helper 失败，它会把本容器改名回去并启动 —— 那时本进程会被重启，状态文件里是 failed。
	res := Result{From: p.From, To: p.To, HelperContainerID: helperID, HandledByHelper: true}
	SetStatus(Status{Phase: PhaseRestarting, From: p.From, To: p.To, BackupPath: helperID, Running: true})
	_ = SaveState(o.StateDir, Status{
		Phase: PhaseRestarting, From: p.From, To: p.To, Image: p.TargetImage, BackupPath: helperID,
	})

	deadline := time.Now().Add(6 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return res, fmt.Errorf("等待 helper 重建容器超时（helper 容器 %s）", helperID)
}

// stateDirFor 返回状态文件目录（与 Options.StateDir 一致；空则退回二进制所在目录）。
func stateDirFor(binaryPath string) string {
	if binaryPath == "" {
		return ""
	}
	return filepath.Dir(binaryPath)
}

// CurrentMode 返回当前进程实际可用的升级方式（不访问网络，只做本地判定）：
// 非容器 → binary / download-only；容器 → 有可用 socket 则 docker-recreate，否则 manual。
// 供 admin 状态接口与前端文案使用，避免前端自己猜。
func CurrentMode(ctx context.Context, socketPath string) Mode {
	env, err := DetectEnvironment("")
	if err != nil {
		return ModeManual
	}
	if !env.Container {
		if env.Systemd && env.DirWritable {
			return ModeBinary
		}
		return ModeDownloadOnly
	}
	if _, err := dockerSocketReady(ctx, socketPath); err == nil {
		return ModeDockerRecreate
	}
	return ModeManual
}
