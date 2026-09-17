package dockerapi

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
)

// Mount 是容器的一个挂载点（用于把宿主路径传给 helper：socket 与数据目录）。
type Mount struct {
	Type        string `json:"Type"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
}

// Container 只保留我们需要的字段。
//
// Config / HostConfig 故意用 map 原样保存：重建容器时要**逐字段沿用**，
// 自己建模整个 schema 只会漏字段（漏一个就可能丢卷、丢端口、丢 restart 策略）。
type Container struct {
	ID              string         `json:"Id"`
	Name            string         `json:"Name"`
	Image           string         `json:"Image"`
	Config          map[string]any `json:"Config"`
	HostConfig      map[string]any `json:"HostConfig"`
	NetworkSettings struct {
		Networks map[string]any `json:"Networks"`
	} `json:"NetworkSettings"`
	Mounts []Mount `json:"Mounts"`
	State  struct {
		Running bool `json:"Running"`
	} `json:"State"`
}

// InspectContainer 读取容器完整信息（id 或名字都可以）。
func (c *Client) InspectContainer(ctx context.Context, idOrName string) (Container, error) {
	var ct Container
	err := c.do(ctx, "GET", "/containers/"+url.PathEscape(idOrName)+"/json", nil, &ct)
	return ct, err
}

// RenameContainer 改名（重建前把旧容器挪开，给新容器腾出原名）。
func (c *Client) RenameContainer(ctx context.Context, id, newName string) error {
	q := url.Values{}
	q.Set("name", strings.TrimPrefix(newName, "/"))
	req, err := c.newRequest(ctx, "POST", "/containers/"+url.PathEscape(id)+"/rename", nil)
	if err != nil {
		return err
	}
	req.URL.RawQuery = q.Encode()
	resp, err := c.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return statusError(resp.StatusCode, msg)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<12))
	return nil
}

// CreateContainer 用 name 与 payload 建容器，返回新容器 ID。
func (c *Client) CreateContainer(ctx context.Context, name string, payload map[string]any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("name", strings.TrimPrefix(name, "/"))
	req, err := c.newRequest(ctx, "POST", "/containers/create", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.URL.RawQuery = q.Encode()
	resp, err := c.httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return "", statusError(resp.StatusCode, msg)
	}
	var out struct {
		ID string `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.do(ctx, "POST", "/containers/"+url.PathEscape(id)+"/start", nil, nil)
}

func (c *Client) StopContainer(ctx context.Context, id string, timeoutSec int) error {
	q := url.Values{}
	q.Set("t", strconv.Itoa(timeoutSec))
	req, err := c.newRequest(ctx, "POST", "/containers/"+url.PathEscape(id)+"/stop", nil)
	if err != nil {
		return err
	}
	req.URL.RawQuery = q.Encode()
	resp, err := c.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// 容器已经停了会返回 304，按成功处理。
	if resp.StatusCode == 304 {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return statusError(resp.StatusCode, msg)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<12))
	return nil
}

func (c *Client) RemoveContainer(ctx context.Context, id string, force bool) error {
	q := url.Values{}
	q.Set("force", strconv.FormatBool(force))
	req, err := c.newRequest(ctx, "DELETE", "/containers/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	req.URL.RawQuery = q.Encode()
	resp, err := c.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return statusError(resp.StatusCode, msg)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<12))
	return nil
}

// WaitContainer 阻塞等待容器结束并返回退出码。
func (c *Client) WaitContainer(ctx context.Context, id string) (int, error) {
	var out struct {
		StatusCode int `json:"StatusCode"`
	}
	if err := c.do(ctx, "POST", "/containers/"+url.PathEscape(id)+"/wait", nil, &out); err != nil {
		return -1, err
	}
	return out.StatusCode, nil
}

// ContainerLogs 取容器日志（stdout+stderr，tail 行数），自动处理 Docker 的多路复用帧。
func (c *Client) ContainerLogs(ctx context.Context, id string, tail int) (string, error) {
	q := url.Values{}
	q.Set("stdout", "1")
	q.Set("stderr", "1")
	q.Set("tail", strconv.Itoa(tail))
	req, err := c.newRequest(ctx, "GET", "/containers/"+url.PathEscape(id)+"/logs", nil)
	if err != nil {
		return "", err
	}
	req.URL.RawQuery = q.Encode()
	resp, err := c.httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return "", statusError(resp.StatusCode, msg)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return demuxLogs(raw), nil
}

// demuxLogs 解析 Docker 日志流：可能是多路复用帧（8 字节头）或裸文本（Tty=true）。
func demuxLogs(raw []byte) string {
	var b strings.Builder
	i := 0
	framed := len(raw) >= 8 && (raw[0] == 0 || raw[0] == 1 || raw[0] == 2) && raw[1] == 0 && raw[2] == 0 && raw[3] == 0
	if !framed {
		return string(raw)
	}
	for i+8 <= len(raw) {
		size := int(binary.BigEndian.Uint32(raw[i+4 : i+8]))
		i += 8
		if size < 0 || i+size > len(raw) {
			break
		}
		b.Write(raw[i : i+size])
		i += size
	}
	return b.String()
}

// RunOnce 跑一个一次性容器（create → start → wait → logs → remove），返回退出码与日志。
// 用于预检（跑新镜像的 --help）。
func (c *Client) RunOnce(ctx context.Context, image string, cmd []string, bindSocket, socketPath string) (int, string, error) {
	payload := map[string]any{
		"Image": image,
		"Cmd":   cmd,
		"HostConfig": map[string]any{
			"AutoRemove":  false,
			"NetworkMode": "none",
		},
	}
	if bindSocket != "" {
		payload["HostConfig"].(map[string]any)["Binds"] = []string{bindSocket + ":" + socketPath}
	}
	id, err := c.CreateContainer(ctx, "", payload)
	if err != nil {
		return -1, "", err
	}
	defer func() { _ = c.RemoveContainer(context.WithoutCancel(ctx), id, true) }()

	if err := c.StartContainer(ctx, id); err != nil {
		return -1, "", err
	}
	code, err := c.WaitContainer(ctx, id)
	logs, _ := c.ContainerLogs(context.WithoutCancel(ctx), id, 200)
	if err != nil {
		return code, logs, err
	}
	return code, logs, nil
}

// Ready 判断 socket 是否可用（Ping + 版本协商）。
func (c *Client) Ready(ctx context.Context) error {
	if err := c.Ping(ctx); err != nil {
		return fmt.Errorf("docker socket 不可用（%s）：%w", c.socketPath, err)
	}
	if _, err := c.ServerVersion(ctx); err != nil {
		return fmt.Errorf("docker 版本探测失败：%w", err)
	}
	return nil
}
