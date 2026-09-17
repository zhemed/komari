// Package dockerapi 是 Docker Engine API 的**最小**客户端（标准库实现，over unix socket）。
//
// 为什么不用 Docker SDK / docker CLI：
//   - 本仓库的依赖面要可控（发布产物要能在 GOPROXY=off 下构建）；
//   - 我们只需要很少几个端点，且必须把"重建容器时保留哪些字段"完全掌握在自己手里。
//
// 用途：容器部署的一键升级（挂 docker.sock 时，拉镜像 + 重建自身容器）。
package dockerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxAPIVersion 是我们声明支持的最高 API 版本；实际用 min(该值, daemon 上报值)。
// 1.41 ≈ Docker 20.10，覆盖面足够，且用到的端点语义稳定。
const maxAPIVersion = "1.41"

const defaultDialTimeout = 5 * time.Second

// Client 通过 unix socket 访问 Docker daemon。
type Client struct {
	socketPath string
	httpc      *http.Client
	api        string // 形如 "1.41"
}

// NewClient 创建客户端；socketPath 为空时用 /var/run/docker.sock。
func NewClient(socketPath string) *Client {
	if strings.TrimSpace(socketPath) == "" {
		socketPath = "/var/run/docker.sock"
	}
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: defaultDialTimeout}
			return d.DialContext(ctx, "unix", socketPath)
		},
		// 不设置代理：unix socket 走不了 HTTP 代理，环境里若有 HTTP_PROXY 会误伤。
		Proxy: nil,
	}
	return &Client{
		socketPath: socketPath,
		httpc:      &http.Client{Transport: tr, Timeout: 0}, // 单请求超时按 ctx 控制（拉镜像可能很久）
		api:        maxAPIVersion,
	}
}

// SocketPath 返回实际使用的 socket 路径。
func (c *Client) SocketPath() string { return c.socketPath }

// APIVersion 返回当前协商到的 API 版本（形如 "1.41"）。
func (c *Client) APIVersion() string { return c.api }

type serverVersion struct {
	APIVersion string `json:"ApiVersion"`
	Version    string `json:"Version"`
}

// ServerVersion 探测 daemon 并协商 API 版本。socket 不存在/无权限时返回错误（调用方据此回落）。
func (c *Client) ServerVersion(ctx context.Context) (serverVersion, error) {
	var v serverVersion
	if err := c.do(ctx, http.MethodGet, "/version", nil, &v); err != nil {
		return v, err
	}
	if v.APIVersion != "" && compareAPIVersion(v.APIVersion, maxAPIVersion) < 0 {
		c.api = v.APIVersion
	}
	return v, nil
}

// Ping 只做一次轻量探测，用于"socket 是否可用"的判定。
func (c *Client) Ping(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, "/_ping", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docker ping: http %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) url(path string, query url.Values) string {
	// Docker 的版本化路径形如 /v1.41/containers/json —— "v" 前缀不能省。
	u := "http://docker/v" + c.api + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.url(path, nil), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// do 执行请求并把 JSON 响应解到 out（out 为 nil 时只校验状态码）。
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return statusError(resp.StatusCode, msg)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// statusError 把 daemon 的错误 JSON（{"message": "..."}）转成可读错误。
func statusError(code int, body []byte) error {
	var payload struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &payload)
	msg := strings.TrimSpace(payload.Message)
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	if msg == "" {
		msg = http.StatusText(code)
	}
	return &APIError{StatusCode: code, Message: msg}
}

// APIError 是 daemon 返回的非 2xx 错误。
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("docker api: http %d: %s", e.StatusCode, e.Message)
}

// IsNotFound / IsConflict / IsForbidden 便于调用方区分处置方式。
func (e *APIError) IsNotFound() bool  { return e.StatusCode == http.StatusNotFound }
func (e *APIError) IsConflict() bool  { return e.StatusCode == http.StatusConflict }
func (e *APIError) IsForbidden() bool { return e.StatusCode == http.StatusForbidden }

// asAPIError 尽力把错误还原成 *APIError。
func asAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if err == nil {
		return nil, false
	}
	if e, ok := err.(*APIError); ok {
		apiErr = e
		return apiErr, true
	}
	return nil, false
}

// compareAPIVersion 比较 "1.41" 形式的版本；a<b 返回 -1。
func compareAPIVersion(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 2; i++ {
		var na, nb int
		if i < len(pa) {
			_, _ = fmt.Sscanf(pa[i], "%d", &na)
		}
		if i < len(pb) {
			_, _ = fmt.Sscanf(pb[i], "%d", &nb)
		}
		if na != nb {
			if na < nb {
				return -1
			}
			return 1
		}
	}
	return 0
}
