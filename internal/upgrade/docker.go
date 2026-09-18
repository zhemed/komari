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

// dockerSocketReady 判断 socket 是否真的可用（存在 + daemon 应答 + 能认出自己所在的容器）。
// 只有真的可用才启用"重建容器"模式；否则一律回落到容器内替换/手工模式。
//
// 识别自身容器用 dockerapi.Client.SelfContainerID：hostname → cgroup → bind 挂载比对。
// 第三条是 2026-09-18 补的：`network_mode: host` 的容器 hostname 是宿主名、cgroup v2 是
// `0::/`，只靠前两条会判不出自己，于是静默退回"容器内替换"（重建容器模式形同虚设）。
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
	selfID, err = client.SelfContainerID(ctx)
	if err != nil {
		return "", err
	}
	return selfID, nil
}

// helperPayload 组装 helper 容器的创建请求。
//
// helper 用**当前镜像**启动（0.0.12 起：本进程所在镜像必然带 docker-self-recreate 子命令；
// 用目标镜像时，降级到 0.0.10 及更早的镜像没有该子命令，helper 会秒退），只挂 docker socket
// 与数据目录（写状态文件用），不挂业务需要的其它卷；**不自动删除**——命名并打标签保留现场，
// 由下一次升级前的清理或人工 `docker logs` 查因。
//
// compose 部署（compose.Usable()）时额外告诉 helper：升级成功后把 compose 文件里本 service
// 的 image tag 同步成新版本（不然文件的 tag 落后，之后任何一次文件编辑都会按旧 tag 把版本拉回去）。
func helperPayload(helperImage, helperName, selfID, targetImage, sanityTag, stateDir, socketPath, binaryPath string, binds []string, compose ComposeInfo) map[string]any {
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
	if compose.Usable() {
		cmd = append(cmd,
			"--compose-files", strings.Join(compose.Files, ","),
			"--compose-service", compose.Service)
	}
	return map[string]any{
		"Image":  helperImage,
		"Cmd":    cmd,
		"Labels": map[string]string{"komari.upgrade.helper": "1", "komari.upgrade.helper.name": helperName},
		"HostConfig": map[string]any{
			// 不自动删除：helper 失败时现场（容器 + 日志）必须留得住，否则用户只看到"没反应"。
			"AutoRemove":    false,
			"NetworkMode":   "none",
			"RestartPolicy": map[string]any{"Name": "no"},
			"Binds":         binds,
		},
	}
}

// helperBinds 计算 helper 需要的卷：docker socket + 状态文件所在的数据目录
// +（compose 部署时）compose 文件所在目录，供升级成功后同步 image tag。
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

	// compose 部署：把 compose 文件所在目录挂进 helper，升级成功后同步 image tag
	//（挂目录而非文件：同步用"写临时文件 + rename"原子替换，rename 到挂载点会失败）。
	binds = append(binds, composeInfoFromContainer(self).DirBinds()...)
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
	// 上一次升级留下的 helper（成功/失败都会留着现场）在这里顺手清掉，避免堆积。
	CleanupHelpers(ctx, socketPath)

	selfImage, _ := self.Config["Image"].(string)
	if strings.TrimSpace(selfImage) == "" {
		selfImage = p.TargetImage
	}
	binds := helperBinds(self, socketPath, o.StateDir)
	helperName := helperContainerName()
	payload := helperPayload(selfImage, helperName, selfID, p.TargetImage, p.To, o.StateDir, socketPath,
		o.ImageBinaryPath, binds, composeInfoFromContainer(self))

	SetStatus(Status{Phase: PhaseReplacing, From: p.From, To: p.To, Detail: "recreating container", Running: true})
	helperID, err := client.CreateContainer(ctx, helperName, payload)
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

	// 监视 helper：它正常工作时会停掉本容器（本进程随之结束）；
	// 若它先退出（失败），必须把原因回报给面板，而不是让用户干等。
	deadline := time.Now().Add(6 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		state, err := client.InspectContainer(ctx, helperID)
		if err != nil {
			continue // 查询失败（例如 daemon 忙）不致命
		}
		if !state.State.Running {
			logs, _ := client.ContainerLogs(context.WithoutCancel(ctx), helperID, 60)
			msg := strings.TrimSpace(logs)
			if msg == "" {
				msg = "helper 容器已退出且没有日志"
			}
			failMsg := fmt.Sprintf("helper 未完成重建（容器 %s）：%s", helperName, lastLines(msg, 6))
			SetStatus(Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: failMsg})
			_ = SaveState(o.StateDir, Status{Phase: PhaseFailed, From: p.From, To: p.To, Error: failMsg})
			return res, fmt.Errorf("%s", failMsg)
		}
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
	}
	return res, fmt.Errorf("等待 helper 重建容器超时（helper 容器 %s）", helperName)
}

// stateDirFor 返回状态文件目录（与 Options.StateDir 一致；空则退回二进制所在目录）。
func stateDirFor(binaryPath string) string {
	if binaryPath == "" {
		return ""
	}
	return filepath.Dir(binaryPath)
}

// helperContainerName 生成 helper 容器名（便于事后 docker logs / 清理）。
func helperContainerName() string {
	return "komari-upgrade-helper-" + time.Now().UTC().Format("20060102-150405")
}

// lastLines 取文本末尾 n 行（日志回报不必刷屏）。
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= n {
		return strings.Join(lines, " | ")
	}
	return strings.Join(lines[len(lines)-n:], " | ")
}

// CleanupHelpers 删除已退出的 helper 容器（新容器启动时调用一次即可）：
// helper 故意不 AutoRemove，所以成功/失败后都会留下现场。
func CleanupHelpers(ctx context.Context, socketPath string) {
	client := dockerapi.NewClient(socketPath)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx); err != nil {
		return
	}
	ids, err := client.ListHelpers(context.WithoutCancel(ctx))
	if err != nil {
		return
	}
	for _, id := range ids {
		_ = client.RemoveContainer(context.WithoutCancel(ctx), id, false)
	}
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
	_ = err
	if _, err := dockerSocketReady(ctx, socketPath); err == nil {
		return ModeDockerRecreate
	}
	if env.DirWritable {
		// 容器里也能在容器内替换二进制（零配置），不是"只能给命令"
		return ModeContainerReplace
	}
	return ModeManual
}
