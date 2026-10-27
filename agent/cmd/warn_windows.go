//go:build windows

package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	toast "gopkg.in/toast.v1"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// ShowToast 在用户态中执行
func ShowToast() {
	title := "Komari is Running"
	message := "The remote control software \"Komari\" is running, which allows others to control your computer. If this was not initiated by you, please terminate the program immediately."

	const aumid = "Komari.Monitor.Agent"
	const linkName = "Komari Warning (Auto Delete Later)"

	if err := ensureStartMenuShortcut(aumid, linkName); err != nil {
		log.Printf("[warn] ensureStartMenuShortcut failed: %v", err)
	}

	n := toast.Notification{
		AppID:   aumid,
		Title:   title,
		Message: message,
		Actions: []toast.Action{
			{Type: "protocol", Label: "Help", Arguments: "https://komari-document.pages.dev/faq/uninstall.html"},
		},
	}
	if err := n.Push(); err != nil {
		log.Printf("[warn] toast push failed: %v", err)
	}

	// 等待 15 秒后删除快捷方式
	shortcutPath := getStartMenuShortcutPath(linkName)
	time.Sleep(15 * time.Second)
	if err := os.Remove(shortcutPath); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[warn] remove shortcut failed: %v", err)
		}
	}
}

// ensureStartMenuShortcut 使用 WScript.Shell 创建 .lnk 并设置 AppUserModelID
func ensureStartMenuShortcut(aumid, linkName string) error {
	programs := getStartMenuProgramsDir()
	if err := os.MkdirAll(programs, 0o755); err != nil {
		return err
	}
	shortcutPath := filepath.Join(programs, sanitizeFileName(linkName)+".lnk")
	if _, err := os.Stat(shortcutPath); err == nil {
		return nil
	}

	if hr := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); hr != nil {
		// S_OK (0) 或 S_FALSE (1) 都视为成功；go-ole 将非零 HRESULT 作为 error 返回
		// 当返回错误时，我们再进行 Uninitialize 保护即可
		// 这里直接继续执行，由后续操作决定是否可用
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return fmt.Errorf("CreateObject WScript.Shell: %w", err)
	}
	defer unknown.Release()

	shell, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("QueryInterface IDispatch: %w", err)
	}
	defer shell.Release()

	cs, err := oleutil.CallMethod(shell, "CreateShortcut", shortcutPath)
	if err != nil {
		return fmt.Errorf("CreateShortcut: %w", err)
	}
	shortcut := cs.ToIDispatch()
	defer shortcut.Release()

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	if _, err = oleutil.PutProperty(shortcut, "TargetPath", exePath); err != nil {
		return fmt.Errorf("set TargetPath: %w", err)
	}
	if _, err = oleutil.PutProperty(shortcut, "WorkingDirectory", exeDir); err != nil {
		return fmt.Errorf("set WorkingDirectory: %w", err)
	}
	_, _ = oleutil.PutProperty(shortcut, "Description", "Komari Agent")
	// 设置 AUMID
	if _, err = oleutil.PutProperty(shortcut, "AppUserModelID", aumid); err != nil {
		// 某些系统该属性不存在时，依然尝试保存；Toast 可能仍然显示
		log.Printf("[warn] set AppUserModelID failed: %v", err)
	}

	if _, err = oleutil.CallMethod(shortcut, "Save"); err != nil {
		return fmt.Errorf("shortcut.Save: %w", err)
	}
	return nil
}

// 返回当前用户开始菜单 Programs 目录
func getStartMenuProgramsDir() string {
	return filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs")
}

// 获取快捷方式完整路径
func getStartMenuShortcutPath(linkName string) string {
	return filepath.Join(getStartMenuProgramsDir(), sanitizeFileName(linkName)+".lnk")
}

func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(name)
}
