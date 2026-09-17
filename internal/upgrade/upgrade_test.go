package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/komari-monitor/komari/internal/dockerapi"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeReleaseServer 提供 releases 列表 + 校验和 + 二进制下载三件套。
type fakeReleaseServer struct {
	tag      string
	payload  []byte
	sums     string // komari-SHA256SUMS 内容
	srv      *httptest.Server
	requests int
}

func newFakeReleaseServer(t *testing.T, tag string, payload []byte, corruptSums bool) *fakeReleaseServer {
	t.Helper()
	f := &fakeReleaseServer{tag: tag, payload: payload}
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])
	if corruptSums {
		hexSum = strings.Repeat("0", 64)
	}
	f.sums = hexSum + "  komari-linux-amd64\n"
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests++
		switch {
		case r.URL.Path == "/repos/owner/repo/releases":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]githubRelease{{
				TagName: tag,
				Assets: []githubAsset{
					{Name: "komari-linux-amd64", BrowserDownloadURL: f.srv.URL + "/dl/" + tag + "/komari-linux-amd64"},
					{Name: ChecksumAssetName, BrowserDownloadURL: f.srv.URL + "/dl/" + tag + "/" + ChecksumAssetName},
				},
			}})
		case strings.HasSuffix(r.URL.Path, "/"+ChecksumAssetName):
			_, _ = w.Write([]byte(f.sums))
		case strings.Contains(r.URL.Path, "/dl/"):
			_, _ = w.Write(f.payload)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeReleaseServer) client() *Client {
	return &Client{Repo: "owner/repo", APIBase: f.srv.URL, HTTP: f.srv.Client()}
}

// 用固定探针替代真实执行：返回目标版本的版本行。
func probeFor(tag string) VersionProbe {
	return func(_ context.Context, path string) (string, error) {
		// 自检要真的执行这个文件：锁住"下载后先补执行位"这条不变量。
		// （2026-09-17 实测踩到：0644 直接 fork/exec → permission denied，升级失败。）
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if info.Mode().Perm()&0o111 == 0 {
			return "", fmt.Errorf("被自检的文件没有执行位: %v", info.Mode().Perm())
		}
		return "[INFO/SERVER] Komari Monitor " + tag + " (hash: deadbeef)", nil
	}
}

func TestExecuteInstallsAndKeepsBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "komari")
	if err := os.WriteFile(binaryPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	fake := newFakeReleaseServer(t, "0.0.8", []byte("new-binary-payload"), false)
	o := Options{
		Repo: "owner/repo", CurrentTag: "0.0.7", BinaryPath: binaryPath, StateDir: dir,
		VersionLister: fake.client(), Probe: probeFor("0.0.8"),
		Env: &Environment{BinaryPath: binaryPath, Systemd: true, DirWritable: true},
	}

	plan, err := Prepare(ctx, o, "")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if plan.To != "0.0.8" || plan.From != "0.0.7" || plan.Manual || plan.DownloadOnly {
		t.Fatalf("计划不符合预期：%+v", plan)
	}

	res, err := Execute(ctx, o, plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// 正式路径已换成新内容，旧内容在备份里
	got, _ := os.ReadFile(binaryPath)
	if string(got) != "new-binary-payload" {
		t.Errorf("二进制未被替换：%q", got)
	}
	backup, _ := os.ReadFile(res.BackupPath)
	if string(backup) != "old-binary" {
		t.Errorf("备份内容不对：%q", backup)
	}
	if res.BackupPath != binaryPath+".backup.0.0.7" {
		t.Errorf("备份路径不符合约定：%s", res.BackupPath)
	}
	// 没有 .part / .tmp 残留
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".part") || strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("有临时文件残留：%s", e.Name())
		}
	}
	// 状态已落盘为 restarting，重启后由 Reconcile 判定完成
	st, _ := LoadState(dir)
	if st.Phase != PhaseRestarting || st.To != "0.0.8" {
		t.Errorf("落盘状态应为 restarting/0.0.8，实际 %+v", st)
	}
	if rec := Reconcile(dir, "0.0.8"); rec.Phase != PhaseCompleted {
		t.Errorf("版本已切换时 Reconcile 应为 completed，实际 %+v", rec)
	}
}

func TestChecksumMismatchKeepsCurrentBinary(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "komari")
	_ = os.WriteFile(binaryPath, []byte("old-binary"), 0o755)
	fake := newFakeReleaseServer(t, "0.0.8", []byte("tampered"), true)
	o := Options{
		Repo: "owner/repo", CurrentTag: "0.0.7", BinaryPath: binaryPath, StateDir: dir,
		VersionLister: fake.client(), Probe: probeFor("0.0.8"),
		Env: &Environment{BinaryPath: binaryPath, Systemd: true, DirWritable: true},
	}
	plan, err := Prepare(ctx, o, "0.0.8")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if _, err := Execute(ctx, o, plan); !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("期望 ErrChecksumMismatch，实际 %v", err)
	}
	if got, _ := os.ReadFile(binaryPath); string(got) != "old-binary" {
		t.Errorf("校验失败后原二进制被改动：%q", got)
	}
	st, _ := LoadState(dir)
	if st.Phase != PhaseFailed || st.Error == "" {
		t.Errorf("失败应落盘 phase=failed 与原因，实际 %+v", st)
	}
}

func TestSanityCheckRejectsWrongVersionBinary(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "komari")
	_ = os.WriteFile(binaryPath, []byte("old-binary"), 0o755)
	fake := newFakeReleaseServer(t, "0.0.8", []byte("payload"), false)
	o := Options{
		Repo: "owner/repo", CurrentTag: "0.0.7", BinaryPath: binaryPath, StateDir: dir,
		VersionLister: fake.client(),
		// 探针说自己是 0.0.6：自检必须拦住，不能替换
		Probe: probeFor("0.0.6"),
		Env:   &Environment{BinaryPath: binaryPath, Systemd: true, DirWritable: true},
	}
	plan, err := Prepare(ctx, o, "0.0.8")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if _, err := Execute(ctx, o, plan); !errors.Is(err, ErrSanityCheck) {
		t.Fatalf("期望 ErrSanityCheck，实际 %v", err)
	}
	if got, _ := os.ReadFile(binaryPath); string(got) != "old-binary" {
		t.Errorf("自检失败后原二进制被改动：%q", got)
	}
}

func TestPrepareContainerReturnsPullCommandOnly(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	fake := newFakeReleaseServer(t, "0.0.8", []byte("payload"), false)
	o := Options{
		Repo: "owner/repo", CurrentTag: "0.0.7", BinaryPath: filepath.Join(dir, "komari"), StateDir: dir,
		VersionLister: fake.client(), Probe: probeFor("0.0.8"),
		Env: &Environment{Container: true, Systemd: true, DirWritable: true},
		// 不存在的 socket：确保走"没挂 socket"的手工分支，不受测试机环境（是否装了 docker）影响
		SocketPath: filepath.Join(dir, "no-such-docker.sock"),
	}
	plan, err := Prepare(ctx, o, "")
	if err != nil {
		t.Fatalf("容器形态不应报错（要给出可操作提示）：%v", err)
	}
	if !plan.Manual || plan.PullCommand != "docker pull ghcr.io/zhemed/komari:0.0.8" {
		t.Fatalf("容器形态应只给拉取命令，实际 %+v", plan)
	}
	res, err := Execute(ctx, o, plan)
	if err != nil || !res.DownloadOnly {
		t.Fatalf("容器形态 Execute 不应做任何替换：%+v %v", res, err)
	}
}

func TestPrepareWithoutSystemdDownloadsOnly(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	fake := newFakeReleaseServer(t, "0.0.8", []byte("payload"), false)
	o := Options{
		Repo: "owner/repo", CurrentTag: "0.0.7", BinaryPath: filepath.Join(dir, "komari"), StateDir: dir,
		VersionLister: fake.client(), Probe: probeFor("0.0.8"),
		Env: &Environment{Systemd: false, DirWritable: true},
	}
	plan, err := Prepare(ctx, o, "0.0.8")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !plan.DownloadOnly || plan.DownloadPath == "" {
		t.Fatalf("无 systemd 时应为仅下载模式，实际 %+v", plan)
	}
	res, err := Execute(ctx, o, plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(res.DownloadPath); err != nil {
		t.Errorf("下载文件不存在：%v", err)
	}
}

func TestPrepareRejectsUnsupportedPlatformAndReadOnlyDir(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	fake := newFakeReleaseServer(t, "0.0.8", []byte("payload"), false)

	o := Options{
		Repo: "owner/repo", CurrentTag: "0.0.7", BinaryPath: filepath.Join(dir, "komari"), StateDir: dir,
		VersionLister: fake.client(), Probe: probeFor("0.0.8"),
		Env: &Environment{Systemd: true, DirWritable: false},
	}
	if _, err := Prepare(ctx, o, "0.0.8"); !errors.Is(err, ErrDirNotWritable) {
		t.Fatalf("目录不可写时应报 ErrDirNotWritable，实际 %v", err)
	}
}

func TestPrepareUpToDate(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	fake := newFakeReleaseServer(t, "0.0.8", []byte("payload"), false)
	o := Options{
		Repo: "owner/repo", CurrentTag: "0.0.8", BinaryPath: filepath.Join(dir, "komari"), StateDir: dir,
		VersionLister: fake.client(), Probe: probeFor("0.0.8"),
		Env: &Environment{Systemd: true, DirWritable: true},
	}
	if _, err := Prepare(ctx, o, ""); !errors.Is(err, ErrUpToDate) {
		t.Fatalf("已是最新时应报 ErrUpToDate，实际 %v", err)
	}
}

func TestFetchChecksumsMissingAssetGivesActionableError(t *testing.T) {
	ctx := context.Background()
	// 模拟 ≤0.0.7 的 release：没有 komari-SHA256SUMS
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]githubRelease{{
			TagName: "0.0.7",
			Assets:  []githubAsset{{Name: "komari-linux-amd64", BrowserDownloadURL: "https://example.invalid/0.0.7"}},
		}})
	}))
	defer srv.Close()
	c := &Client{Repo: "owner/repo", APIBase: srv.URL, HTTP: srv.Client()}
	r, err := c.FindRelease(ctx, "0.0.7")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.FetchChecksums(ctx, r)
	if err == nil || !strings.Contains(err.Error(), "install-komari.sh") {
		t.Fatalf("缺校验和资产时要给出可操作的提示，实际 %v", err)
	}
}

// 容器里没挂可用的 docker socket 时必须回落到手工模式（不能试图重建容器）。
func TestPrepareContainerWithoutUsableSocketStaysManual(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	fake := newFakeReleaseServer(t, "0.0.8", []byte("payload"), false)
	o := Options{
		Repo: "owner/repo", CurrentTag: "0.0.7", BinaryPath: filepath.Join(dir, "komari"), StateDir: dir,
		VersionLister: fake.client(), Probe: probeFor("0.0.8"),
		Env:        &Environment{Container: true, Systemd: true, DirWritable: true},
		SocketPath: filepath.Join(dir, "missing.sock"),
	}
	plan, err := Prepare(ctx, o, "0.0.8")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if plan.Mode != ModeManual || !plan.Manual {
		t.Fatalf("无 socket 时应为手工模式，实际 mode=%s manual=%v", plan.Mode, plan.Manual)
	}
	if !strings.Contains(plan.PullCommand, "docker pull ") {
		t.Fatalf("手工模式要给出可复制命令：%q", plan.PullCommand)
	}
}

// helper 容器的参数与挂载必须正确：用目标镜像、带重建参数、只挂 socket 与数据目录。
func TestHelperPayloadAndBinds(t *testing.T) {
	self := dockerapi.Container{
		ID:   "abcdefabcdefabcdef",
		Name: "/komari",
		Mounts: []dockerapi.Mount{
			{Type: "bind", Source: "/srv/komari/data", Destination: "/app/data"},
			{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock"},
		},
	}
	binds := helperBinds(self, DefaultDockerSocket, "/app/data")
	if len(binds) != 2 {
		t.Fatalf("应挂 socket 与数据目录，实际 %v", binds)
	}
	joined := strings.Join(binds, ",")
	if !strings.Contains(joined, "/var/run/docker.sock:/var/run/docker.sock") {
		t.Fatalf("socket 挂载缺失：%v", binds)
	}
	if !strings.Contains(joined, "/srv/komari/data:/app/data") {
		t.Fatalf("数据目录应用宿主路径挂载：%v", binds)
	}

	payload := helperPayload("ghcr.io/zhemed/komari:0.0.11", self.ID, "ghcr.io/zhemed/komari:0.0.11",
		"0.0.11", "/app/data", DefaultDockerSocket, "", binds)
	cmd, _ := payload["Cmd"].([]string)
	joinedCmd := strings.Join(cmd, " ")
	for _, want := range []string{"docker-self-recreate", "--container abcdefabcdefabcdef", "--image ghcr.io/zhemed/komari:0.0.11", "--sanity-tag 0.0.11", "--state-dir /app/data"} {
		if !strings.Contains(joinedCmd, want) {
			t.Errorf("helper 命令缺少 %q：%s", want, joinedCmd)
		}
	}
	hc, _ := payload["HostConfig"].(map[string]any)
	if hc["AutoRemove"] != true || hc["NetworkMode"] != "none" {
		t.Fatalf("helper 的 HostConfig 不符合预期：%+v", hc)
	}
}

// 数据目录没挂出来时不应硬塞一个错误的卷映射。
func TestHelperBindsSkipsUnmountedStateDir(t *testing.T) {
	self := dockerapi.Container{
		Mounts: []dockerapi.Mount{{Type: "bind", Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock"}},
	}
	binds := helperBinds(self, DefaultDockerSocket, "/app/data")
	if len(binds) != 1 {
		t.Fatalf("只应挂 socket，实际 %v", binds)
	}
}
