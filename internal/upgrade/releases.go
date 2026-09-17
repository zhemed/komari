// Package upgrade 实现服务器自升级：查询 release、下载并校验资产、原子替换二进制。
//
// 设计约束（见 .trellis/tasks/09-17-panel-one-click-upgrade/design.md）：
//   - 只用标准库，不引入第三方自更新库（减少供应链面，且校验和/备份/容器分支都要自己控）；
//   - 目标仓库由调用方（服务端设置）提供，**绝不接受请求方传入的 URL**；
//   - 非 linux 平台没有发布资产：直接报错，不做半途尝试。
package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultAPIBase 是 GitHub API 基址；测试用 httptest 覆盖。
	DefaultAPIBase = "https://api.github.com"
	// ChecksumAssetName 是服务端二进制的校验和资产名（与 agent 的 komari-agent-SHA256SUMS 分开）。
	ChecksumAssetName = "komari-SHA256SUMS"
	// userAgent 必须设置，否则 GitHub API 会拒绝请求。
	userAgent = "komari-self-upgrade"
)

// Asset 是 release 里的一个资产。
type Asset struct {
	Name string
	URL  string
}

// Release 是升级所需的 release 元数据（只保留必要字段）。
type Release struct {
	Tag         string
	Name        string
	PublishedAt time.Time
	Prerelease  bool
	Assets      map[string]Asset
}

// Asset 按名字取资产。
func (r Release) Asset(name string) (Asset, bool) {
	a, ok := r.Assets[name]
	return a, ok
}

// Client 访问 GitHub release API。
type Client struct {
	Repo    string // owner/repo
	HTTP    *http.Client
	APIBase string // 空则用 DefaultAPIBase
	Token   string // 可选；私有仓库或限流时使用
}

func (c *Client) base() string {
	if c.APIBase != "" {
		return strings.TrimRight(c.APIBase, "/")
	}
	return DefaultAPIBase
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []githubAsset `json:"assets"`
}

func (c *Client) fetch(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github api %s: http %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ListReleases 返回全部 release（含 prerelease），按版本从高到低排序。
func (c *Client) ListReleases(ctx context.Context) ([]Release, error) {
	if c.Repo == "" {
		return nil, fmt.Errorf("release repo is not configured")
	}
	var raw []githubRelease
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=100", c.base(), c.Repo)
	if err := c.fetch(ctx, url, &raw); err != nil {
		return nil, err
	}
	releases := make([]Release, 0, len(raw))
	for _, r := range raw {
		if r.Draft {
			continue
		}
		assets := make(map[string]Asset, len(r.Assets))
		for _, a := range r.Assets {
			assets[a.Name] = Asset{Name: a.Name, URL: a.BrowserDownloadURL}
		}
		tag := strings.TrimSpace(r.TagName)
		releases = append(releases, Release{
			Tag:         tag,
			Name:        r.Name,
			PublishedAt: r.PublishedAt,
			Prerelease:  r.Prerelease,
			Assets:      assets,
		})
	}
	sort.SliceStable(releases, func(i, j int) bool {
		if c, ok := CompareVersions(releases[i].Tag, releases[j].Tag); ok {
			if c != 0 {
				return c > 0
			}
		}
		return releases[i].PublishedAt.After(releases[j].PublishedAt)
	})
	return releases, nil
}

// StableReleases 只返回稳定版（跳过 prerelease），已按版本降序。
func (c *Client) StableReleases(ctx context.Context) ([]Release, error) {
	all, err := c.ListReleases(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Release, 0, len(all))
	for _, r := range all {
		if r.Prerelease {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// LatestStable 返回最新的稳定版；若它不比 current 新，ok=false。
func (c *Client) LatestStable(ctx context.Context, current string) (Release, bool, error) {
	list, err := c.StableReleases(ctx)
	if err != nil {
		return Release{}, false, err
	}
	if len(list) == 0 {
		return Release{}, false, fmt.Errorf("no stable release found in %s", c.Repo)
	}
	latest := list[0]
	cmp, ok := CompareVersions(latest.Tag, current)
	if !ok {
		// 版本号无法比较时保守处理：只要 tag 不同就认为有新版（面板已有版本提示，用户自己决定）。
		return latest, latest.Tag != current, nil
	}
	return latest, cmp > 0, nil
}

// FindRelease 精确查找某个 tag（用于"安装指定版本"/回滚）。
func (c *Client) FindRelease(ctx context.Context, tag string) (Release, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return Release{}, fmt.Errorf("empty tag")
	}
	list, err := c.ListReleases(ctx)
	if err != nil {
		return Release{}, err
	}
	for _, r := range list {
		if r.Tag == tag {
			return r, nil
		}
	}
	return Release{}, fmt.Errorf("release %s not found in %s", tag, c.Repo)
}

// ServerAssetName 返回当前平台对应的服务端资产名（与 scripts/build-komari.sh 的产物名一致）。
func ServerAssetName(goos, goarch string) (string, bool) {
	if goos != "linux" {
		return "", false
	}
	switch goarch {
	case "amd64", "arm64":
		return "komari-" + goos + "-" + goarch, true
	default:
		return "", false
	}
}

// ParseVersion 解析 x.y.z（允许 "v" 前缀与前后空白）；tag 与 CurrentVersion 都走这里。
func ParseVersion(v string) (major, minor, patch int, ok bool) {
	s := strings.TrimSpace(v)
	s = strings.TrimPrefix(s, "v")
	// 只取前三段，忽略可能出现的后缀（例如 0.0.7-rc1 中的 -rc1 会被截断后忽略）。
	base := s
	if i := strings.IndexAny(base, "-+"); i >= 0 {
		base = base[:i]
	}
	parts := strings.Split(base, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, false
	}
	nums := make([]int, 0, 3)
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 {
			return 0, 0, 0, false
		}
		nums = append(nums, n)
	}
	for len(nums) < 3 {
		nums = append(nums, 0)
	}
	return nums[0], nums[1], nums[2], true
}

// CompareVersions 比较两个版本号：-1 表示 a<b，1 表示 a>b，0 表示相等。
// ok=false 表示至少一个版本号无法解析（调用方据此选择保守策略，不要当成 0）。
func CompareVersions(a, b string) (int, bool) {
	am, an, ap, ok1 := ParseVersion(a)
	bm, bn, bp, ok2 := ParseVersion(b)
	if !ok1 || !ok2 {
		return 0, false
	}
	switch {
	case am != bm:
		return sign(am - bm), true
	case an != bn:
		return sign(an - bn), true
	case ap != bp:
		return sign(ap - bp), true
	default:
		return 0, true
	}
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}

// ParseChecksums 解析 `sha256  filename` 形式的校验和清单（兼容 `*filename` 二进制模式标记）。
func ParseChecksums(data []byte) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		sum := strings.ToLower(fields[0])
		if len(sum) != 64 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		out[name] = sum
	}
	return out
}
