package admin

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/komari-monitor/komari/database/models"
)

// TestIsValidThemeShort_PathTraversal 防止 DeleteTheme/UpdateTheme/SetTheme
// 的路径穿越漏洞：req.Short 直接进入 filepath.Join("./data/theme", short)，
// 若 short 含 "../" 会规范化到 ./data/theme 之外，配合 os.RemoveAll 可删除
// 工作目录外任意目录。isValidThemeShort 必须拒绝所有此类 payload。
func TestIsValidThemeShort_PathTraversal(t *testing.T) {
	// 必须被拒绝：路径穿越 / 绝对路径 / 目录分隔符 / 空值 / default
	deny := []string{
		"",
		"default",
		"..",
		"../",
		"./",
		"../../etc",
		"../..",
		"foo/../bar",
		"/etc/passwd",
		"foo/bar",
		"foo\\bar",
		"a b",
		"a;b",
		"a$(id)",
	}
	for _, in := range deny {
		if isValidThemeShort(in) {
			t.Errorf("isValidThemeShort(%q) = true, want false (路径穿越/非法字符未被拦截)", in)
		}
	}

	// 必须被接受：仅字母数字下划线连字符
	accept := []string{
		"mytheme",
		"my-theme",
		"my_theme",
		"theme123",
		"ABC",
		"a",
	}
	for _, in := range accept {
		if !isValidThemeShort(in) {
			t.Errorf("isValidThemeShort(%q) = false, want true (合法名称被误拒)", in)
		}
	}
}

func TestValidateThemeManifestAcceptsLocalizedMetadata(t *testing.T) {
	theme := models.Theme{
		Name: map[string]any{
			"zh-CN": "配置字段演示主题",
			"en":    "Managed Configuration Demo Theme",
		},
		Short: "managed-config-demo",
		Description: map[string]any{
			"zh-CN": "用于验证全部托管配置字段的测试主题。",
			"en":    "A test theme covering every managed configuration field type.",
		},
		Author: map[string]any{
			"zh-CN": "Komari 团队",
			"en":    "Komari",
		},
	}

	if err := validateThemeManifest(theme); err != nil {
		t.Fatalf("localized theme metadata was rejected: %v", err)
	}
}

func TestPeekThemeFromZipAcceptsLocalizedMetadata(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "localized-theme.zip")
	archive, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(archive)
	manifest, err := writer.Create("komari-theme.json")
	if err != nil {
		t.Fatalf("create manifest: %v", err)
	}
	_, err = manifest.Write([]byte(`{
  "name": {"zh-CN": "本地化主题", "en": "Localized Theme"},
  "short": "localized-theme",
  "description": {"zh-CN": "描述", "en": "Description"},
  "author": {"zh-CN": "作者", "en": "Author"},
  "version": "1.0.0"
}`))
	if err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}

	theme, err := peekThemeFromZip(zipPath)
	if err != nil {
		t.Fatalf("localized theme package was rejected: %v", err)
	}
	if theme.Short != "localized-theme" {
		t.Fatalf("short = %q, want localized-theme", theme.Short)
	}

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(workDir)
	})

	installed, err := extractAndValidateTheme(zipPath)
	if err != nil {
		t.Fatalf("localized theme upload package was rejected: %v", err)
	}
	if installed.Short != "localized-theme" {
		t.Fatalf("installed short = %q, want localized-theme", installed.Short)
	}
}

// TestIsGitHubRepoURL 防止把 GitHub release 下载 / archive / tree 等具体资源 URL
// 误判成仓库根地址。2026-11-12 本机实例实测：主题导入/更新把
// https://github.com/zhemed/komari/releases/download/theme-latest/ink-theme.zip
// 当成仓库地址，去 GitHub API 取 latest release 的第一个资产（服务器二进制），
// 解压时报 "not a valid zip"。修复后只有 /owner/repo[.git][/] 才算仓库地址。
func TestIsGitHubRepoURL(t *testing.T) {
	cases := []struct {
		name  string
		url   string
		want  bool
		owner string
		repo  string
	}{
		{"repo root", "https://github.com/owner/repo", true, "owner", "repo"},
		{"repo root with .git", "https://github.com/owner/repo.git", true, "owner", "repo"},
		{"repo root trailing slash", "https://github.com/owner/repo/", true, "owner", "repo"},
		{"www host", "https://www.github.com/owner/repo", true, "owner", "repo"},
		{"http scheme", "http://github.com/owner/repo", true, "owner", "repo"},
		{"release asset download", "https://github.com/zhemed/komari/releases/download/theme-latest/ink-theme.zip", false, "", ""},
		{"release page", "https://github.com/owner/repo/releases", false, "", ""},
		{"archive zip", "https://github.com/owner/repo/archive/refs/heads/main.zip", false, "", ""},
		{"tree page", "https://github.com/owner/repo/tree/main", false, "", ""},
		{"blob page", "https://github.com/owner/repo/blob/main/README.md", false, "", ""},
		{"owner only", "https://github.com/owner", false, "", ""},
		{"github root", "https://github.com/", false, "", ""},
		{"non github host", "https://gitlab.com/owner/repo", false, "", ""},
		{"empty", "", false, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, owner, repo := isGitHubRepoURL(tc.url)
			if got != tc.want || owner != tc.owner || repo != tc.repo {
				t.Fatalf("isGitHubRepoURL(%q) = (%v, %q, %q), want (%v, %q, %q)",
					tc.url, got, owner, repo, tc.want, tc.owner, tc.repo)
			}
		})
	}
}
