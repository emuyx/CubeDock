package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	nemcraknet "github.com/Happy2018new/nemc-tan-lobby-solver/core/raknet"
	goraknet "github.com/sandertv/go-raknet"

	"github.com/emuyx/cubedock/internal/bridge"
	"github.com/emuyx/cubedock/internal/config"
)

type limboProcess struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	cleanup func()
	done    chan struct{}
	once    sync.Once
}

// limboGroup tracks one profile's limbo process and how many running rooms use
// it. The process is stopped only after the last reference is released.
type limboGroup struct {
	process  *limboProcess
	refs     int
	starting bool
	wait     chan struct{}
	err      error
}

type limboLease struct {
	manager   *Manager
	profileID string
	once      sync.Once
}

// Release gives up one room's use of the profile's limbo process.
func (l *limboLease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() { l.manager.releaseLimbo(l.profileID) })
}

func limboExecutable(limboPath string) (string, error) {
	path := strings.TrimSpace(limboPath)
	if path == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		name := "limbo"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		path = filepath.Join(filepath.Dir(exe), name)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("limbo 路径无效: %w", err)
	}
	if runtime.GOOS == "windows" && strings.HasPrefix(filepath.VolumeName(absolute), `\\`) {
		return "", fmt.Errorf("limbo 路径不能是网络共享")
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", fmt.Errorf("未找到 limbo 辅助进程 %s: %w", absolute, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("limbo 路径不是普通文件: %s", absolute)
	}
	return absolute, nil
}

func safeProfileID(id string) string {
	var builder strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	value := builder.String()
	if value == "" || value == "." || value == ".." {
		return "default"
	}
	return value
}

// ProfilePacksDir returns the resource pack directory for one profile. An
// explicit directory wins; otherwise packs live under the data directory so
// different upstream servers do not mix their packs.
func ProfilePacksDir(dataDir string, profile config.Profile) string {
	if dir := strings.TrimSpace(profile.LimboPacksDir); dir != "" {
		if absolute, err := filepath.Abs(dir); err == nil {
			return absolute
		}
		return dir
	}
	return filepath.Join(dataDir, "packs", safeProfileID(profile.ID))
}

// EnsureLimboPacksDir creates the profile's resource pack directory.
func EnsureLimboPacksDir(dataDir string, profile config.Profile) (string, error) {
	dir := ProfilePacksDir(dataDir, profile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return dir, fmt.Errorf("创建资源包目录 %s 失败: %w", dir, err)
	}
	return dir, nil
}

// limboOutput splits limbo's stdout/stderr into log lines and forwards selected
// lines to the console event stream.
type limboOutput struct {
	logger *log.Logger
	mu     sync.Mutex
	buf    []byte
	onLine func(string)
}

func (o *limboOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buf = append(o.buf, p...)
	for {
		index := bytes.IndexByte(o.buf, '\n')
		if index < 0 {
			break
		}
		line := strings.TrimSpace(string(o.buf[:index]))
		o.buf = o.buf[index+1:]
		if line == "" {
			continue
		}
		if o.logger != nil {
			o.logger.Print(line)
		}
		if o.onLine != nil {
			o.onLine(line)
		}
	}
	if len(o.buf) > 64*1024 {
		o.buf = o.buf[len(o.buf)-64*1024:]
	}
	return len(p), nil
}

func startLimboProcess(dataDir string, profile config.Profile, limboPath string, logger *log.Logger, onLine func(string)) (*limboProcess, error) {
	path, err := limboExecutable(limboPath)
	if err != nil {
		return nil, err
	}
	// The pack directory may have been changed after startup; create it before
	// limbo is launched so an explicit path never points at a missing folder.
	packsDir, err := EnsureLimboPacksDir(dataDir, profile)
	if err != nil {
		return nil, err
	}
	args := []string{
		"-listen", profile.LimboUpstream,
		"-target", profile.Upstream,
		"-watch-parent=true",
		"-packs", packsDir,
		"-form-title", profile.LimboFormTitle,
		"-form-content", profile.LimboFormContent,
		"-form-button", profile.LimboFormButton,
	}
	cmd := exec.Command(path, args...)
	cmd.Env = childEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	if logger != nil || onLine != nil {
		output := &limboOutput{logger: logger, onLine: onLine}
		cmd.Stdout = output
		cmd.Stderr = output
	}
	configureLimboCmd(cmd)
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("启动 limbo 失败: %w", err)
	}
	cleanup, err := attachLimboJob(cmd)
	if err != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("绑定 limbo 生命周期失败: %w", err)
	}
	process := &limboProcess{cmd: cmd, stdin: stdin, cleanup: cleanup, done: make(chan struct{})}
	go func() {
		err := cmd.Wait()
		close(process.done)
		if err != nil && logger != nil {
			logger.Printf("[limbo] 辅助进程已退出: %v", err)
		}
	}()

	deadline := time.Now().Add(6 * time.Second)
	for {
		if _, err := goraknet.PingTimeout(profile.LimboUpstream, 400*time.Millisecond); err == nil {
			return process, nil
		}
		select {
		case <-process.done:
			process.stop()
			return nil, fmt.Errorf("limbo 辅助进程启动后立即退出")
		case <-time.After(100 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			process.stop()
			return nil, fmt.Errorf("limbo 辅助进程未在 %s 响应", profile.LimboUpstream)
		}
	}
}

// ensureLimboProcess starts the profile's limbo if it is not running. It does
// not change the reference count; callers that own a room use acquireLimbo.
func (m *Manager) ensureLimboProcess(profile config.Profile) error {
	m.mu.Lock()
	limboPath := m.cfg.LimboPath
	group := m.limbos[profile.ID]
	if group != nil && group.process != nil {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	m.limboMu.Lock()
	defer m.limboMu.Unlock()

	m.mu.Lock()
	group = m.limbos[profile.ID]
	if group != nil && group.process != nil {
		m.mu.Unlock()
		return nil
	}
	if group == nil {
		group = &limboGroup{wait: make(chan struct{})}
		m.limbos[profile.ID] = group
	}
	group.starting = true
	group.err = nil
	group.wait = make(chan struct{})
	m.mu.Unlock()

	process, err := m.startLimboFn(m.dir, profile, limboPath, m.logger, func(line string) {
		m.recordLimboLine(profile.Name, line)
	})

	m.mu.Lock()
	group.starting = false
	if err != nil {
		group.err = err
		close(group.wait)
		m.mu.Unlock()
		return err
	}
	group.process = process
	close(group.wait)
	m.mu.Unlock()
	go m.watchLimbo(profile.ID, group, process)
	return nil
}

func (m *Manager) watchLimbo(profileID string, group *limboGroup, process *limboProcess) {
	<-process.done
	m.mu.Lock()
	if group.process == process {
		group.process = nil
	}
	m.mu.Unlock()
}

// acquireLimbo registers one room's use of a profile's limbo process, starting
// the process when needed.
func (m *Manager) acquireLimbo(profile config.Profile) (*limboLease, error) {
	for {
		m.mu.Lock()
		group := m.limbos[profile.ID]
		if group != nil && group.starting {
			wait := group.wait
			m.mu.Unlock()
			<-wait
			continue
		}
		if group != nil && group.process != nil {
			group.refs++
			m.mu.Unlock()
			return &limboLease{manager: m, profileID: profile.ID}, nil
		}
		if group != nil && group.err != nil {
			// Retry a previous automatic start failure instead of pinning
			// the profile to a stale error forever.
			group.err = nil
			m.mu.Unlock()
			if err := m.ensureLimboProcess(profile); err != nil {
				return nil, err
			}
			continue
		}
		m.mu.Unlock()
		if err := m.ensureLimboProcess(profile); err != nil {
			return nil, err
		}
	}
}

func (m *Manager) releaseLimbo(profileID string) {
	m.mu.Lock()
	group := m.limbos[profileID]
	if group == nil {
		m.mu.Unlock()
		return
	}
	if group.refs > 0 {
		group.refs--
	}
	var process *limboProcess
	if group.refs == 0 && !group.starting {
		process = group.process
		delete(m.limbos, profileID)
	}
	m.mu.Unlock()
	if process != nil {
		process.stop()
	}
}

// stopAllLimbos kills every helper, used when all rooms are stopped or the
// process exits.
func (m *Manager) stopAllLimbos() {
	m.limboMu.Lock()
	m.mu.Lock()
	groups := m.limbos
	m.limbos = make(map[string]*limboGroup)
	m.mu.Unlock()
	for _, group := range groups {
		if group.process != nil {
			group.process.stop()
		}
	}
	m.limboMu.Unlock()
}

// recordLimboLine surfaces important limbo diagnostics in the console without
// forwarding the routine per-connection logs.
func (m *Manager) recordLimboLine(profileName, line string) {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "login"), strings.Contains(lower, "encryption"),
		strings.Contains(lower, "failed"), strings.Contains(lower, "panic"),
		strings.Contains(lower, "accept:"), strings.Contains(lower, "start game:"):
		m.record("limbo", line, false, 0, profileName)
	}
}

func dialUpstream(ctx context.Context, profile config.Profile) (relayStream, error) {
	if profile.ConnectionMode == config.ConnectionTransfer {
		return goraknet.DialContext(ctx, profile.LimboUpstream)
	}
	return nemcraknet.DialContext(ctx, profile.Upstream)
}

// relayStream is the subset of relay.Stream used by dialUpstream.
type relayStream = interface {
	ReadPacket() ([]byte, error)
	Write([]byte) (int, error)
	Close() error
}

func upstreamAddress(profile config.Profile) string {
	if profile.ConnectionMode == config.ConnectionTransfer {
		return profile.LimboUpstream
	}
	return profile.Upstream
}

func upstreamDescription(profile config.Profile) string {
	if profile.ConnectionMode == config.ConnectionTransfer {
		return fmt.Sprintf("limbo %s → %s", profile.LimboUpstream, profile.Upstream)
	}
	return profile.Upstream
}

func pingUpstream(ctx context.Context, profile config.Profile) (string, error) {
	if profile.ConnectionMode == config.ConnectionTransfer {
		// go-raknet's PingContext may wait indefinitely on a silent endpoint.
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if _, err := goraknet.PingContext(pingCtx, profile.LimboUpstream); err != nil {
			return "", err
		}
		return "limbo", nil
	}
	return bridge.PingServer(ctx, profile.Upstream)
}

func (p *limboProcess) stop() {
	if p == nil {
		return
	}
	p.once.Do(func() {
		if p.stdin != nil {
			_ = p.stdin.Close()
		}
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		if p.cleanup != nil {
			p.cleanup()
		}
		select {
		case <-p.done:
		case <-time.After(3 * time.Second):
		}
	})
}
