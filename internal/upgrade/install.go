package upgrade

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	containerMarkerPath = "/.dockerenv"
	systemdRuntimePath  = "/run/systemd/system"
)

var (
	// ErrContainer 表示运行在容器里：替换二进制会随容器重建丢失，必须改拉镜像。
	ErrContainer = errors.New("running inside a container")
	// ErrNotSupervised 表示没有服务管理器接手：直接退出会丢服务，只能下载后手工替换。
	ErrNotSupervised = errors.New("process is not supervised by systemd")
	// ErrDirNotWritable 表示二进制所在目录不可写，无法做原子替换。
	ErrDirNotWritable = errors.New("binary directory is not writable")
	// ErrSanityCheck 表示下载下来的二进制没有通过自检（版本行不符或无法执行）。
	ErrSanityCheck = errors.New("downloaded binary failed sanity check")
)

// Environment 描述当前部署形态。
type Environment struct {
	Container   bool
	Systemd     bool
	BinaryPath  string
	DirWritable bool
}

// DetectEnvironment 探测部署形态。binaryPath 为空时用 os.Executable()。
func DetectEnvironment(binaryPath string) (Environment, error) {
	if binaryPath == "" {
		exe, err := os.Executable()
		if err != nil {
			return Environment{}, err
		}
		binaryPath = exe
	}
	env := Environment{BinaryPath: binaryPath}
	if _, err := os.Stat(containerMarkerPath); err == nil {
		env.Container = true
	}
	if _, err := os.Stat(systemdRuntimePath); err == nil {
		env.Systemd = true
	}
	env.DirWritable = dirWritable(filepath.Dir(binaryPath))
	return env, nil
}

// dirWritable 用"创建再删除探测文件"判断目录可写（只读挂载与权限不足都会失败）。
func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".komari-write-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// PullHint 返回容器形态下的镜像更新命令。
func PullHint(image, tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("docker pull %s:%s", image, tag)
}

// VersionProbe 读取出二进制自报的版本输出。
type VersionProbe func(ctx context.Context, path string) (string, error)

// ProbeVersion 默认探针：跑 `<path> --help`。
//
// 注意（实测）：本项目的二进制**没有** `--version` flag（会以退出码 1 + "unknown flag" 结束），
// 但版本行在任何调用时都会打印，`--help` 退出码为 0，因此用它做无害自检。
func ProbeVersion(ctx context.Context, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: 执行 %s 失败: %v", ErrSanityCheck, filepath.Base(path), err)
	}
	return string(out), nil
}

// VerifyBinary 用 probe 自检下载下来的二进制：输出必须包含 "Komari Monitor <tag>"。
// 这是替换前的最后一道闸门——不通过就绝不覆盖正在运行的二进制。
func VerifyBinary(ctx context.Context, probe VersionProbe, path, tag string) error {
	if probe == nil {
		probe = ProbeVersion
	}
	out, err := probe(ctx, path)
	if err != nil {
		return err
	}
	want := "Komari Monitor " + strings.TrimSpace(tag)
	if !strings.Contains(out, want) {
		return fmt.Errorf("%w: 输出里没有 %q", ErrSanityCheck, want)
	}
	return nil
}

// SwapBinary 原子替换：先把现有二进制改名为备份，再把新文件改名到正式路径。
//
// 两步 rename 之间若进程被杀，磁盘上仍有可用备份（backup 路径随返回值/错误给出）。
func SwapBinary(binaryPath, newPath, currentTag string) (backupPath string, err error) {
	currentTag = strings.TrimSpace(currentTag)
	if currentTag == "" {
		currentTag = "unknown"
	}
	backupPath = fmt.Sprintf("%s.backup.%s", binaryPath, currentTag)
	_ = os.Remove(backupPath) // 同一版本重复升级时覆盖旧备份，避免 rename 因目标存在失败
	if err := os.Chmod(newPath, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(binaryPath, backupPath); err != nil {
		return "", fmt.Errorf("备份现有二进制失败: %w", err)
	}
	if err := os.Rename(newPath, binaryPath); err != nil {
		// 尽力恢复原状，避免服务起不来。
		_ = os.Rename(backupPath, binaryPath)
		return "", fmt.Errorf("替换二进制失败（已尝试回滚）: %w", err)
	}
	return backupPath, nil
}
