package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/internal/dockerapi"
	"github.com/komari-monitor/komari/internal/upgrade"
	"github.com/spf13/cobra"
)

// docker-self-recreate 是"容器一键升级"的 helper：由旧容器里的服务端启动它，
// 它自己跑在**独立容器**里，所以能安全地把旧容器停掉并用新镜像重建。
//
// 本子命令也可以手工执行用于恢复：
//
//	komari docker-self-recreate --container komari --image ghcr.io/zhemed/komari:0.0.11 \
//	  --sanity-tag 0.0.11 --state-dir /app/data
//
// 退出码：0 成功；1 失败（失败时旧容器通常已被回滚启动）。
var (
	dockerSelfRecreateContainer string
	dockerSelfRecreateImage     string
	dockerSelfRecreateSanityTag string
	dockerSelfRecreateStateDir  string
	dockerSelfRecreateSocket    string
	dockerSelfRecreateTimeout   time.Duration
	// compose 部署时由服务端传入：升级成功后把 compose 文件里的 image tag 同步成新版本。
	dockerSelfRecreateComposeFiles   string
	dockerSelfRecreateComposeService string
)

// syncComposeTag 把 compose 文件里本 service 的 image tag 同步成目标版本。
//
// 只在收到 --compose-files/--compose-service 时执行；任何失败都只记日志，
// **不影响升级结果**（容器已经重建成功，这里只是消除"文件 tag 落后"的后续陷阱）。
func syncComposeTag() {
	if dockerSelfRecreateComposeService == "" || strings.TrimSpace(dockerSelfRecreateComposeFiles) == "" {
		return
	}
	for _, file := range strings.Split(dockerSelfRecreateComposeFiles, ",") {
		if file = strings.TrimSpace(file); file == "" {
			continue
		}
		res, err := upgrade.SyncComposeImage(file, dockerSelfRecreateComposeService, dockerSelfRecreateImage)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "[docker-self-recreate] compose 同步失败（不影响升级）: %v\n", err)
			continue
		}
		if res.Changed {
			_, _ = fmt.Fprintf(os.Stdout, "[docker-self-recreate] compose synced: %s  %s -> %s（备份 %s）\n",
				res.File, res.OldImage, res.NewImage, res.BackupPath)
			return
		}
		_, _ = fmt.Fprintf(os.Stdout, "[docker-self-recreate] compose 跳过 %s：%s\n", res.File, res.Reason)
	}
}

var DockerSelfRecreateCmd = &cobra.Command{
	Use:   "docker-self-recreate",
	Short: "Recreate this container with a new image (panel one-click upgrade helper)",
	Long: `用目标镜像重建指定的容器：读取旧容器配置 → 预检新镜像 → 旧容器改名 →
建同名新容器 → 停旧 → 起新。任何一步失败都会把旧容器改名回去并启动。

这是面板"一键升级"在容器部署下使用的 helper；手工执行可用于故障恢复。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dockerSelfRecreateContainer == "" || dockerSelfRecreateImage == "" {
			return fmt.Errorf("--container 与 --image 必填")
		}
		ctx, cancel := context.WithTimeout(context.Background(), dockerSelfRecreateTimeout)
		defer cancel()

		client := dockerapi.NewClient(dockerSelfRecreateSocket)
		if err := client.Ready(ctx); err != nil {
			return err
		}

		writeState := func(phase upgrade.Phase, err error, res dockerapi.RecreateResult, digest string) {
			if dockerSelfRecreateStateDir == "" {
				return
			}
			st := upgrade.Status{
				Phase:      phase,
				To:         dockerSelfRecreateSanityTag,
				Image:      dockerSelfRecreateImage,
				Digest:     digest,
				BackupPath: res.OldContainerName,
			}
			if err != nil {
				st.Error = err.Error()
			}
			_ = upgrade.SaveState(dockerSelfRecreateStateDir, st)
		}

		res, err := client.Recreate(ctx, dockerapi.RecreateOptions{
			Container: dockerSelfRecreateContainer,
			NewImage:  dockerSelfRecreateImage,
			SanityTag: dockerSelfRecreateSanityTag,
			Progress: func(phase, detail string) {
				_, _ = fmt.Fprintf(os.Stdout, "[docker-self-recreate] %s %s\n", phase, detail)
			},
		})
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "[docker-self-recreate] failed: %v\n", err)
			writeState(upgrade.PhaseFailed, err, res, "")
			return err
		}
		digest, _ := client.ImageDigest(ctx, dockerSelfRecreateImage)
		if res.Noop {
			_, _ = fmt.Fprintln(os.Stdout, "[docker-self-recreate] image already matches, nothing to do")
			writeState(upgrade.PhaseCompleted, nil, res, digest)
			return nil
		}
		_, _ = fmt.Fprintf(os.Stdout, "[docker-self-recreate] done: new=%s old=%s digest=%s\n",
			res.NewContainerID, res.OldContainerName, digest)
		syncComposeTag()
		writeState(upgrade.PhaseCompleted, nil, res, digest)
		return nil
	},
}

func init() {
	DockerSelfRecreateCmd.Flags().StringVar(&dockerSelfRecreateContainer, "container", "", "要重建的容器 ID 或名字")
	DockerSelfRecreateCmd.Flags().StringVar(&dockerSelfRecreateImage, "image", "", "目标镜像（repo:tag）")
	DockerSelfRecreateCmd.Flags().StringVar(&dockerSelfRecreateSanityTag, "sanity-tag", "", "预检要求 --help 输出里出现的版本号")
	DockerSelfRecreateCmd.Flags().StringVar(&dockerSelfRecreateStateDir, "state-dir", "", "升级状态文件目录（容器内的数据目录）")
	DockerSelfRecreateCmd.Flags().StringVar(&dockerSelfRecreateSocket, "socket", "", "docker socket 路径（默认 /var/run/docker.sock）")
	DockerSelfRecreateCmd.Flags().DurationVar(&dockerSelfRecreateTimeout, "timeout", 10*time.Minute, "整体超时")
	DockerSelfRecreateCmd.Flags().StringVar(&dockerSelfRecreateComposeFiles, "compose-files", "", "compose 部署：配置文件路径（逗号分隔，升级成功后同步 image tag）")
	DockerSelfRecreateCmd.Flags().StringVar(&dockerSelfRecreateComposeService, "compose-service", "", "compose 部署：本容器所属 service 名")
	_ = flags.DatabaseFile // 本子命令不碰数据库，保持与其它子命令一致的包依赖
	RootCmd.AddCommand(DockerSelfRecreateCmd)
}
