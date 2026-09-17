package upgrade

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Phase 是升级流程的阶段，前端据此展示进度。
type Phase string

const (
	PhaseIdle        Phase = "idle"
	PhaseDownloading Phase = "downloading"
	PhaseVerifying   Phase = "verifying"
	PhaseReplacing   Phase = "replacing"
	PhaseRestarting  Phase = "restarting"
	PhaseFailed      Phase = "failed"
	PhaseCompleted   Phase = "completed"
)

// Status 是升级状态。进程重启后仍能读到（落盘在 data/upgrade-state.json），
// 因此面板在重启完成后可以显示"上次升级的结果"。
type Status struct {
	Phase Phase  `json:"phase"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
	Error string `json:"error,omitempty"`
	// BackupPath：二进制模式 = 旧二进制的备份路径；docker 模式 = 旧容器的名字（保留为回滚点）。
	BackupPath string `json:"backup_path,omitempty"`
	// Image / Digest 仅 docker 模式使用：目标镜像与拉取到的 digest（审计）。
	Image  string `json:"image,omitempty"`
	Digest string `json:"digest,omitempty"`
	// Detail 是给面板看的细节（如拉取进度、"recreating container"）。
	Detail     string    `json:"detail,omitempty"`
	UpdatesDir string    `json:"updates_dir,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Running 表示当前进程里是否真的有一个升级任务在跑（不落盘）。
	Running bool `json:"running"`
}

const stateFileName = "upgrade-state.json"

var (
	statusMu sync.RWMutex
	status   Status
)

// SetStatus 更新内存态（同时刷新时间戳）。
func SetStatus(s Status) {
	s.UpdatedAt = time.Now().UTC()
	statusMu.Lock()
	status = s
	statusMu.Unlock()
}

// StatusSnapshot 读取内存态；若内存态为空则回退到磁盘上的上次结果。
func StatusSnapshot() Status {
	statusMu.RLock()
	s := status
	statusMu.RUnlock()
	if s.Phase == "" {
		s.Phase = PhaseIdle
	}
	return s
}

// MarkRunning 设置/清除"正在升级"标记。
func MarkRunning(running bool) {
	statusMu.Lock()
	if running {
		status.Running = true
	} else {
		status.Running = false
	}
	statusMu.Unlock()
}

// IsRunning 返回是否已有升级任务在跑（防止并发触发）。
func IsRunning() bool {
	statusMu.RLock()
	defer statusMu.RUnlock()
	return status.Running
}

// StatePath 返回状态文件路径。
func StatePath(stateDir string) string {
	return filepath.Join(stateDir, stateFileName)
}

// LoadState 读取落盘状态；文件不存在时返回 PhaseIdle 与 nil 错误。
func LoadState(stateDir string) (Status, error) {
	var s Status
	if stateDir == "" {
		return s, nil
	}
	data, err := os.ReadFile(StatePath(stateDir))
	if err != nil {
		if os.IsNotExist(err) {
			s.Phase = PhaseIdle
			return s, nil
		}
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Status{Phase: PhaseIdle}, err
	}
	if s.Phase == "" {
		s.Phase = PhaseIdle
	}
	return s, nil
}

// SaveState 原子写状态文件（tmp + rename），保证重启后读到的是完整 JSON。
func SaveState(stateDir string, s Status) error {
	if stateDir == "" {
		return nil
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = time.Now().UTC()
	}
	s.Running = false
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := StatePath(stateDir) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, StatePath(stateDir))
}
