package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// ErrChecksumMismatch 表示下载内容与发布清单不一致：必须拒绝安装并保留现场以外的原状。
var ErrChecksumMismatch = errors.New("checksum mismatch")

// MaxAssetBytes 是单个资产的下载上限（服务端静态二进制约 35MB，留足余量防跑飞）。
const MaxAssetBytes = 256 << 20

// FetchChecksums 下载并解析校验和资产；缺失该资产的 release（≤0.0.7）返回明确错误。
func (c *Client) FetchChecksums(ctx context.Context, r Release) (map[string]string, error) {
	asset, ok := r.Asset(ChecksumAssetName)
	if !ok {
		return nil, fmt.Errorf("release %s 没有 %s 资产（该版本不支持一键升级，请用 install-komari.sh 升级）",
			r.Tag, ChecksumAssetName)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载 %s 失败：http %d", ChecksumAssetName, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	sums := ParseChecksums(data)
	if len(sums) == 0 {
		return nil, fmt.Errorf("%s 内容不可解析", ChecksumAssetName)
	}
	return sums, nil
}

// DownloadVerified 把 url 下载到 dst，边下边算 SHA256 并与 wantSHA 比对。
// 任何失败（网络中断、超过上限、校验不符）都会删除 dst，绝不留下半截文件。
func (c *Client) DownloadVerified(ctx context.Context, url, dst, wantSHA string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载 %s 失败：http %d", filepath.Base(dst), resp.StatusCode)
	}

	tmp := dst + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(f, hash), io.LimitReader(resp.Body, MaxAssetBytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if written > MaxAssetBytes {
		_ = os.Remove(tmp)
		return fmt.Errorf("资产超过上限 %d 字节，已中止", int64(MaxAssetBytes))
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if wantSHA != "" && got != wantSHA {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: 期望 %s，实际 %s", ErrChecksumMismatch, wantSHA, got)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
