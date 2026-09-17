package jsonrpc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/komari-monitor/komari/utils"
)

func TestValidateUpgradeRepo(t *testing.T) {
	valid := []string{"zhemed/komari", "owner/repo-name", "a/b_c.d"}
	for _, r := range valid {
		if err := validateUpgradeRepo(r); err != nil {
			t.Errorf("%q 应通过校验，实际 %v", r, err)
		}
	}
	// 重点是挡住 URL 与注入式内容：目标仓库不接受请求方传入的任意地址。
	invalid := []string{"", "zhemed", "zhemed/komari/extra", "/komari", "zhemed/", "https://github.com/zhemed/komari",
		"zhemed/komari?foo=1", "zhemed/komari\nother", "zhemed/../komari"}
	for _, r := range invalid {
		if err := validateUpgradeRepo(r); err == nil {
			t.Errorf("%q 应被拒绝", r)
		}
	}
}

func TestBuildUpgradeOptionsUsesExecutableDir(t *testing.T) {
	opts, err := buildUpgradeOptions(serverUpgradeSettings{Repo: "zhemed/komari"})
	if err != nil {
		t.Fatalf("buildUpgradeOptions: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if opts.BinaryPath != exe {
		t.Errorf("BinaryPath = %q，期望 %q", opts.BinaryPath, exe)
	}
	if opts.StateDir != filepath.Dir(exe) {
		t.Errorf("StateDir = %q，期望二进制同目录 %q", opts.StateDir, filepath.Dir(exe))
	}
	if opts.CurrentTag != utils.CurrentVersion {
		t.Errorf("CurrentTag = %q，期望 %q", opts.CurrentTag, utils.CurrentVersion)
	}
}
