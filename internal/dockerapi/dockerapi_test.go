package dockerapi

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeDaemon 是一个跑在 unix socket 上的假 Docker daemon，
// 这样测试走的是**真实的传输路径**（unix dial + HTTP + 流式响应）。
type fakeDaemon struct {
	mu       sync.Mutex
	calls    []string
	payloads []map[string]any

	container  Container
	pullError  string
	failCreate bool
	failStart  bool
	helpOutput string
	newImageID string
	// listItems 是 GET /containers/json 的返回（识别自身容器时用）。
	listItems []ContainerSummary
}

func (f *fakeDaemon) record(msg string) {
	f.calls = append(f.calls, msg)
}

func (f *fakeDaemon) called(substr string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.Contains(c, substr) {
			return true
		}
	}
	return false
}

func newTestClient(t *testing.T) (*Client, *fakeDaemon) {
	t.Helper()
	f := &fakeDaemon{
		helpOutput: "Komari Monitor 0.0.11 (hash: deadbeef)",
		newImageID: "newcontainerid",
	}
	f.container = Container{
		ID:    "abcdefabcdefabcdef",
		Name:  "/komari",
		Image: "ghcr.io/zhemed/komari:0.0.10",
		Config: map[string]any{
			"Image":      "ghcr.io/zhemed/komari:0.0.10",
			"Hostname":   "abcdefabcdef",
			"Env":        []any{"TZ=Asia/Shanghai"},
			"Entrypoint": nil,
			"Cmd":        []any{"/app/komari", "server"},
			"Labels":     map[string]any{"com.example": "1"},
			"WorkingDir": "/app",
		},
		HostConfig: map[string]any{
			"Binds":         []any{"/srv/komari/data:/app/data", "/var/run/docker.sock:/var/run/docker.sock"},
			"NetworkMode":   "bridge",
			"RestartPolicy": map[string]any{"Name": "always", "MaximumRetryCount": float64(0)},
			"PortBindings":  map[string]any{"25774/tcp": []any{map[string]any{"HostIp": "", "HostPort": "25774"}}},
		},
	}
	f.container.NetworkSettings.Networks = map[string]any{"bridge": map[string]any{"Aliases": []any{"komari"}}}

	mux := http.NewServeMux()
	handle := func(pattern string, fn func(w http.ResponseWriter, r *http.Request)) {
		mux.HandleFunc(pattern, fn)
	}
	handle("/_ping", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("OK")) })
	handle("/version", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"ApiVersion": "1.41", "Version": "24.0.7"})
	})
	// 容器列表：识别自身容器（bind 挂载比对）要用。
	handle("/containers/json", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.record("list:all=" + r.URL.Query().Get("all"))
		items := f.listItems
		f.mu.Unlock()
		if items == nil {
			items = []ContainerSummary{}
		}
		_ = json.NewEncoder(w).Encode(items)
	})
	handle("/images/create", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.record("pull:" + r.URL.Query().Get("fromImage") + ":" + r.URL.Query().Get("tag"))
		pullErr := f.pullError
		f.mu.Unlock()
		flusher, _ := w.(http.Flusher)
		for _, s := range []string{"Pulling fs layer", "Downloading", "Extracting"} {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": s})
			if flusher != nil {
				flusher.Flush()
			}
		}
		if pullErr != "" {
			_ = json.NewEncoder(w).Encode(map[string]any{"error": pullErr})
		}
	})
	handle("/containers/create", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		name := r.URL.Query().Get("name")
		f.mu.Lock()
		f.payloads = append(f.payloads, payload)
		f.record("create:" + name)
		// failCreate 只作用于"具名创建"（= 真正的重建），预检容器（无名）必须能跑起来，
		// 否则测不到回滚路径。
		fail := f.failCreate && name != ""
		f.mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "create boom"})
			return
		}
		id := f.newImageID
		if name == "" {
			id = "precheckcontainer"
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"Id": id})
	})
	handle("/containers/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/containers/")
		parts := strings.Split(rest, "/")
		id := parts[0]
		action := ""
		if len(parts) > 1 {
			action = parts[1]
		}
		if r.Method == http.MethodDelete {
			f.mu.Lock()
			f.record("remove:" + id)
			f.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		f.mu.Lock()
		failStart := f.failStart && id == f.newImageID
		f.mu.Unlock()
		switch {
		case action == "":
			_ = json.NewEncoder(w).Encode(f.container)
		case action == "json":
			_ = json.NewEncoder(w).Encode(f.container)
		case action == "rename":
			f.mu.Lock()
			f.record("rename:" + id + "->" + r.URL.Query().Get("name"))
			newName := r.URL.Query().Get("name")
			f.mu.Unlock()
			if id == f.container.ID {
				f.container.Name = "/" + newName
			}
			w.WriteHeader(http.StatusNoContent)
		case action == "start":
			f.mu.Lock()
			f.record("start:" + id)
			f.mu.Unlock()
			if failStart && id == f.newImageID {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "start boom"})
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case action == "stop":
			f.mu.Lock()
			f.record("stop:" + id)
			f.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case action == "wait":
			_ = json.NewEncoder(w).Encode(map[string]int{"StatusCode": 0})
		case action == "logs":
			writeFramed(w, []byte(f.helpOutput))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/containers/"+f.container.ID, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			f.mu.Lock()
			f.record("remove:" + strings.TrimPrefix(r.URL.Path, "/containers/"))
			f.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(f.container)
	})

	sock := filepath.Join(t.TempDir(), "docker.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	// 真实 daemon 同时接受带/不带版本前缀的路径；这里统一剥掉 "/v1.xx"。
	stripVersion := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1.") {
			if i := strings.Index(r.URL.Path[1:], "/"); i >= 0 {
				r.URL.Path = r.URL.Path[i+1:]
			}
		}
		mux.ServeHTTP(w, r)
	})
	srv := &http.Server{Handler: stripVersion}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return NewClient(sock), f
}

// writeFramed 按 Docker 的日志多路复用格式写出（8 字节头 + 负载）。
func writeFramed(w io.Writer, payload []byte) {
	header := make([]byte, 8)
	header[0] = 1
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
	_, _ = w.Write(header)
	_, _ = w.Write(payload)
}

func TestPingAndVersionNegotiation(t *testing.T) {
	c, _ := newTestClient(t)
	ctx := context.Background()
	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	v, err := c.ServerVersion(ctx)
	if err != nil {
		t.Fatalf("ServerVersion: %v", err)
	}
	if v.Version != "24.0.7" || c.APIVersion() != "1.41" {
		t.Fatalf("版本协商异常：%+v api=%s", v, c.APIVersion())
	}
}

func TestPullImageProgressAndStreamError(t *testing.T) {
	c, f := newTestClient(t)
	ctx := context.Background()
	var seen []string
	if err := c.PullImage(ctx, "ghcr.io/zhemed/komari:0.0.11", func(s string) { seen = append(seen, s) }); err != nil {
		t.Fatalf("PullImage: %v", err)
	}
	if len(seen) < 3 {
		t.Fatalf("应收到多条进度，实际 %v", seen)
	}
	if !f.called("pull:ghcr.io/zhemed/komari:0.0.11") {
		t.Fatalf("拉取参数不对：%v", f.calls)
	}
	// daemon 把错误放在 200 的流里：必须能识别
	f.pullError = "manifest unknown"
	err := c.PullImage(ctx, "ghcr.io/zhemed/komari:9.9.9", nil)
	if err == nil || !strings.Contains(err.Error(), "manifest unknown") {
		t.Fatalf("流内错误未被识别：%v", err)
	}
}

func TestParseImageRef(t *testing.T) {
	cases := []struct {
		ref  string
		repo string
		tag  string
	}{
		{"ghcr.io/zhemed/komari:0.0.11", "ghcr.io/zhemed/komari", "0.0.11"},
		{"ghcr.io/zhemed/komari", "ghcr.io/zhemed/komari", "latest"},
		{"localhost:5000/komari:dev", "localhost:5000/komari", "dev"},
		{"komari@sha256:abc", "komari@sha256:abc", ""},
	}
	for _, c := range cases {
		got := ParseImageRef(c.ref)
		if got.Repo != c.repo || got.Tag != c.tag {
			t.Errorf("ParseImageRef(%q) = (%q,%q)，期望 (%q,%q)", c.ref, got.Repo, got.Tag, c.repo, c.tag)
		}
	}
	if got := ParseImageRef("ghcr.io/zhemed/komari:0.0.10").WithTag("0.0.11"); got != "ghcr.io/zhemed/komari:0.0.11" {
		t.Errorf("WithTag = %q", got)
	}
}

func TestBuildRecreatePayloadPreservesConfigAndReplacesImage(t *testing.T) {
	c, _ := newTestClient(t)
	old, err := c.InspectContainer(context.Background(), "komari")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	payload, err := BuildRecreatePayload(old, "ghcr.io/zhemed/komari:0.0.11", "komari")
	if err != nil {
		t.Fatalf("BuildRecreatePayload: %v", err)
	}
	if payload["Image"] != "ghcr.io/zhemed/komari:0.0.11" {
		t.Fatalf("镜像未被替换：%v", payload["Image"])
	}
	// Hostname 等于旧容器短 ID → 必须删掉，交给 daemon 重新分配
	if _, exists := payload["Hostname"]; exists {
		t.Fatalf("等于容器短 ID 的 Hostname 应被清除：%v", payload["Hostname"])
	}
	// 关键配置必须逐项保留
	hc, ok := payload["HostConfig"].(map[string]any)
	if !ok {
		t.Fatalf("HostConfig 丢失")
	}
	binds, _ := json.Marshal(hc["Binds"])
	if !strings.Contains(string(binds), "/app/data") || !strings.Contains(string(binds), "docker.sock") {
		t.Fatalf("卷未保留：%s", binds)
	}
	if hc["NetworkMode"] != "bridge" {
		t.Fatalf("网络模式未保留：%v", hc["NetworkMode"])
	}
	if _, ok := hc["RestartPolicy"]; !ok {
		t.Fatalf("restart 策略未保留")
	}
	if _, ok := hc["PortBindings"]; !ok {
		t.Fatalf("端口映射未保留")
	}
	env, _ := json.Marshal(payload["Env"])
	if !strings.Contains(string(env), "TZ=Asia/Shanghai") {
		t.Fatalf("env 未保留：%s", env)
	}
	nc, ok := payload["NetworkingConfig"].(map[string]any)
	if !ok {
		t.Fatalf("网络别名未还原")
	}
	if !strings.Contains(fmt.Sprint(nc), "komari") {
		t.Fatalf("网络别名内容不对：%v", nc)
	}
}

func TestRecreateHappyPath(t *testing.T) {
	c, f := newTestClient(t)
	ctx := context.Background()
	var phases []string
	res, err := c.Recreate(ctx, RecreateOptions{
		Container: "komari",
		NewImage:  "ghcr.io/zhemed/komari:0.0.11",
		SanityTag: "0.0.11",
		Progress:  func(phase, detail string) { phases = append(phases, phase) },
	})
	if err != nil {
		t.Fatalf("Recreate: %v", err)
	}
	if res.NewContainerID != "newcontainerid" || res.OldContainerName == "" {
		t.Fatalf("结果不对：%+v", res)
	}
	if !strings.HasPrefix(res.OldContainerName, "komari-old-") {
		t.Fatalf("旧容器命名不符合约定：%s", res.OldContainerName)
	}
	for _, want := range []string{"rename:", "create:komari", "stop:", "start:newcontainerid"} {
		if !f.called(want) {
			t.Errorf("缺少调用 %s（实际 %v）", want, f.calls)
		}
	}
	// 顺序：改名 → 建新 → 停旧 → 起新
	idx := func(sub string) int {
		for i, c := range f.calls {
			if strings.Contains(c, sub) {
				return i
			}
		}
		return -1
	}
	if !(idx("rename:") < idx("create:komari") && idx("create:komari") < idx("stop:") && idx("stop:") < idx("start:newcontainerid")) {
		t.Fatalf("调用顺序不对：%v", f.calls)
	}
	if len(phases) == 0 || phases[0] != "precheck" {
		t.Fatalf("阶段不符合预期：%v", phases)
	}
}

func TestRecreateRollsBackWhenCreateFails(t *testing.T) {
	c, f := newTestClient(t)
	f.failCreate = true
	_, err := c.Recreate(context.Background(), RecreateOptions{
		Container: "komari", NewImage: "ghcr.io/zhemed/komari:0.0.11", SanityTag: "0.0.11",
	})
	if err == nil || !strings.Contains(err.Error(), "创建新容器失败") {
		t.Fatalf("应返回创建失败，实际 %v", err)
	}
	// 回滚：把旧容器改名回去 + 启动它；且从未去停/删旧容器
	if !f.called("rename:abcdefabcdefabcdef->komari") {
		t.Fatalf("未回滚改名：%v", f.calls)
	}
	if !f.called("start:abcdefabcdefabcdef") {
		t.Fatalf("未回滚启动旧容器：%v", f.calls)
	}
	if f.called("stop:") {
		t.Fatalf("失败路径不该停旧容器：%v", f.calls)
	}
}

func TestRecreateRollsBackWhenStartFails(t *testing.T) {
	c, f := newTestClient(t)
	f.failStart = true
	_, err := c.Recreate(context.Background(), RecreateOptions{
		Container: "komari", NewImage: "ghcr.io/zhemed/komari:0.0.11", SanityTag: "0.0.11",
	})
	if err == nil || !strings.Contains(err.Error(), "启动新容器失败") {
		t.Fatalf("应返回启动失败，实际 %v", err)
	}
	if !f.called("remove:newcontainerid") {
		t.Fatalf("半成品容器应被删除：%v", f.calls)
	}
	if !f.called("start:abcdefabcdefabcdef") {
		t.Fatalf("应回滚启动旧容器：%v", f.calls)
	}
}

func TestRecreateRejectsBadImageBeforeTouchingContainer(t *testing.T) {
	c, f := newTestClient(t)
	f.helpOutput = "Komari Monitor 0.0.10 (hash: other)" // 预检版本行不对
	_, err := c.Recreate(context.Background(), RecreateOptions{
		Container: "komari", NewImage: "ghcr.io/zhemed/komari:0.0.11", SanityTag: "0.0.11",
	})
	if err == nil || !strings.Contains(err.Error(), "预检失败") {
		t.Fatalf("应因预检失败中止，实际 %v", err)
	}
	// 预检本身会跑一次性容器，但不能碰旧容器
	if f.called("rename:") || f.called("stop:") || f.called("create:komari") {
		t.Fatalf("预检失败时不该动被升级的容器：%v", f.calls)
	}
}

func TestRecreateNoopWhenImageUnchanged(t *testing.T) {
	c, f := newTestClient(t)
	res, err := c.Recreate(context.Background(), RecreateOptions{
		Container: "komari", NewImage: "ghcr.io/zhemed/komari:0.0.10", SanityTag: "0.0.10",
	})
	if err != nil {
		t.Fatalf("同名镜像应视为 no-op：%v", err)
	}
	if !res.Noop {
		t.Fatalf("应标记 no-op：%+v", res)
	}
	if len(f.calls) != 0 { // inspect 不计入 calls
		t.Fatalf("no-op 路径不该有其他调用：%v", f.calls)
	}
}

func TestDetectSelfContainerIDShape(t *testing.T) {
	id := DetectSelfContainerID()
	if id == "" {
		t.Skip("当前环境拿不到容器 ID（非容器或 hostname/cgroup 都不含 ID），跳过")
	}
	if !reShortID.MatchString(id) && !reLongID.MatchString(id) {
		t.Fatalf("返回的 ID 形状不对：%q", id)
	}
}

func TestDemuxLogs(t *testing.T) {
	var buf bytes.Buffer
	writeFramed(&buf, []byte("hello world"))
	if got := demuxLogs(buf.Bytes()); got != "hello world" {
		t.Fatalf("demux 结果 = %q", got)
	}
	if got := demuxLogs([]byte("plain text")); got != "plain text" {
		t.Fatalf("裸文本应原样返回：%q", got)
	}
}
