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
	// 注意：**不要**用 os.Executable() 做等值校验。二进制替换是"旧文件改名成备份 + 新文件占位"，
	// 而 /proc/self/exe 跟随 inode —— 替换后它指向的是备份文件（实测踩到，导致每次都误判失败）。
	// 这里只确认目标文件存在且可执行，然后 exec 它；版本正确性已由替换前的自检保证。
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("新二进制不可用（%s）：%w", binaryPath, err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("新二进制没有执行位：%s（%v）", binaryPath, info.Mode().Perm())
	}
	if err := syscall.Exec(binaryPath, os.Args, os.Environ()); err != nil {
		return fmt.Errorf("exec %s 失败：%w", binaryPath, err)
	}
	return nil // 正常情况下不会到这里
}
