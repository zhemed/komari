package upgrade

import (
	"fmt"
	"os"
	"syscall"
)

// SelfExec 用新二进制**原地替换当前进程**（execve）：PID 不变、容器不重启、
// 不依赖 systemd 或 docker 的 restart 策略。
//
// 这是容器零配置升级的关键：不挂 docker socket 的容器里，替换完二进制后
// 直接把当前进程换成新版本即可；调用方在 SelfExec 返回错误时应回落为退出进程
// （由 docker 的 restart 策略拉起）。
//
// 注意：Go 的监听 socket 都是 CLOEXEC 的，exec 后旧监听自动关闭，
// 新进程可以重新绑定同一端口。
func SelfExec(binaryPath string) error {
	if binaryPath == "" {
		return fmt.Errorf("empty binary path")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe != binaryPath {
		// 允许传路径做校验，但真正 exec 的是"被替换后的那个文件"，两者应一致；
		// 不一致说明调用方弄错了路径，宁可报错也不要执行错的文件。
		return fmt.Errorf("SelfExec 路径不一致：参数 %s，当前可执行文件 %s", binaryPath, exe)
	}
	if err := syscall.Exec(binaryPath, os.Args, os.Environ()); err != nil {
		return fmt.Errorf("exec %s 失败：%w", binaryPath, err)
	}
	return nil // 正常情况下不会到这里
}
