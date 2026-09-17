package dockerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// ImageRef 解析 "ghcr.io/owner/name:tag" 形式的引用。
type ImageRef struct {
	// Repo 是不含 tag 的部分（含 registry host 与 namespace）。
	Repo string
	// Tag 是标签；为空表示 latest。
	Tag string
}

// ParseImageRef 拆出 repo 与 tag。只处理 "repo:tag" 形式（我们自己的镜像形态），
// 带 digest（@sha256:…）的引用按原样当作 Repo，Tag 为空。
func ParseImageRef(ref string) ImageRef {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ImageRef{}
	}
	if strings.Contains(ref, "@") {
		return ImageRef{Repo: ref}
	}
	// 最后一段里找 ":"（避免把 registry 的端口号当分隔符）
	slash := strings.LastIndex(ref, "/")
	colon := strings.LastIndex(ref, ":")
	if colon > slash {
		return ImageRef{Repo: ref[:colon], Tag: ref[colon+1:]}
	}
	return ImageRef{Repo: ref, Tag: "latest"}
}

// WithTag 返回把 tag 替换为 tag 之后的完整引用。
func (r ImageRef) WithTag(tag string) string {
	if r.Repo == "" {
		return ""
	}
	if tag == "" {
		tag = "latest"
	}
	return r.Repo + ":" + tag
}

// PullProgress 是一次拉取进度回调（status 形如 "Pulling fs layer"、"Downloading"）。
type PullProgress func(status string)

// PullImage 拉取镜像；进度流式回调。daemon 的错误也会以 200 + {"error":...} 的形式出现在流里，
// 所以必须逐条读流而不是只看状态码。
func (c *Client) PullImage(ctx context.Context, ref string, onProgress PullProgress) error {
	parsed := ParseImageRef(ref)
	q := url.Values{}
	q.Set("fromImage", parsed.Repo)
	if parsed.Tag != "" {
		q.Set("tag", parsed.Tag)
	}
	req, err := c.newRequest(ctx, "POST", "/images/create", nil)
	if err != nil {
		return err
	}
	req.URL.RawQuery = q.Encode()
	// 私有仓库需要 X-Registry-Auth；本项目的镜像公开，匿名拉取即可。
	resp, err := c.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		return statusError(resp.StatusCode, msg)
	}

	dec := json.NewDecoder(resp.Body)
	last := ""
	for dec.More() {
		var item struct {
			Status string `json:"status"`
			ID     string `json:"id"`
			Error  string `json:"error"`
			Detail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if err := dec.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if item.Error != "" {
			msg := item.Error
			if item.Detail.Message != "" {
				msg = item.Detail.Message
			}
			return fmt.Errorf("拉取镜像失败：%s", msg)
		}
		line := strings.TrimSpace(item.Status)
		if item.ID != "" {
			line = item.ID + ": " + line
		}
		if onProgress != nil && line != "" && line != last {
			last = line
			onProgress(line)
		}
	}
	return nil
}

// ImageDigest 返回本地镜像的 digest（用于审计：升级到哪个 digest）。
func (c *Client) ImageDigest(ctx context.Context, ref string) (string, error) {
	var info struct {
		ID          string   `json:"Id"`
		RepoDigests []string `json:"RepoDigests"`
	}
	if err := c.do(ctx, "GET", "/images/"+url.PathEscape(ref)+"/json", nil, &info); err != nil {
		return "", err
	}
	for _, d := range info.RepoDigests {
		if i := strings.Index(d, "@"); i >= 0 {
			return d[i+1:], nil
		}
	}
	return info.ID, nil
}
