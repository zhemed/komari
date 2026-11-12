package public

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/komari-monitor/komari/internal/config"
)

// BuiltinThemeShort 是随二进制内置的主题 short 名。
const BuiltinThemeShort = "ink"

// builtinThemeArchivePath 是内置主题包在仓库/二进制内的路径。
// 该 zip 是 themes/ink 的构建产物（构建脚本同步，提交进仓库），随 go:embed 进二进制；
// 全新部署时无需再手动安装主题。
const builtinThemeArchivePath = "builtinThemes/ink.zip"

//go:embed builtinThemes/ink.zip
var builtinThemeArchives embed.FS

// InstallBuiltinThemes 在 data/theme/<short> 缺失或损坏（缺 komari-theme.json）时，
// 从二进制内置主题包恢复。已有安装（含用户通过主题渠道更新的版本）不覆盖。
func InstallBuiltinThemes() error {
	targetDir := filepath.Join(DataDir, ThemesDir, BuiltinThemeShort)
	if themeManifestExists(targetDir) {
		return nil
	}

	data, err := builtinThemeArchives.ReadFile(builtinThemeArchivePath)
	if err != nil {
		return fmt.Errorf("read builtin theme archive: %w", err)
	}
	if err := extractThemeZip(data, targetDir); err != nil {
		return fmt.Errorf("install builtin theme %q: %w", BuiltinThemeShort, err)
	}
	if !themeManifestExists(targetDir) {
		return fmt.Errorf("builtin theme %q archive is missing komari-theme.json", BuiltinThemeShort)
	}
	return nil
}

// ActivateBuiltinThemeIfFresh 在主题配置从未设置过时，把默认主题设为内置主题。
// 已有显式设置（包括 "default"）不覆盖——升级不改变老实例的主题选择。
func ActivateBuiltinThemeIfFresh() error {
	if _, err := config.GetAs[string](config.ThemeKey); err == nil {
		return nil
	}
	return config.Set(config.ThemeKey, BuiltinThemeShort)
}

func themeManifestExists(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "komari-theme.json"))
	return err == nil && !info.IsDir()
}

// extractThemeZip 把主题 zip 安全解压到 targetDir（拒绝路径穿越；忽略符号链接）。
func extractThemeZip(data []byte, targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	// 只有 manifest 缺失时才会走到这里，清掉可能残留的半份目录再重建。
	if err := os.RemoveAll(targetDir); err != nil {
		return err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}

	cleanTarget := filepath.Clean(targetDir)
	for _, file := range reader.File {
		name := filepath.Clean(strings.TrimPrefix(file.Name, "/"))
		if name == "" || name == "." || name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
			continue
		}
		dest := filepath.Join(cleanTarget, name)
		if dest != cleanTarget && !strings.HasPrefix(dest, cleanTarget+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry escapes target dir: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := writeZipFile(file, dest); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(file *zip.File, dest string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
