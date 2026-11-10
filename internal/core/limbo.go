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
	"github.com/emuyx/cubedock/internal/relay"
)

type limboProcess struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	cleanup func()
	done    chan struct{}
	once    sync.Once
}

func limboExecutable(cfg config.Config) (string, error) {
	path := cfg.LimboPath
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

// DefaultLimboPacksDir returns where CubeDock expects limbo resource packs.
// limbo_packs_dir wins; otherwise the directory next to limbo.exe (or the main
// executable) is used, matching limbo's own auto-discovery.
func DefaultLimboPacksDir(cfg config.Config) string {
	if dir := strings.TrimSpace(cfg.LimboPacksDir); dir != "" {
		if absolute, err := filepath.Abs(dir); err == nil {
			return absolute
		}
		return dir
	}
	base := ""
	if path := strings.TrimSpace(cfg.LimboPath); path != "" {
		base = filepath.Dir(path)
	} else if exe, err := os.Executable(); err == nil {
		base = filepath.Dir(exe)
	}
	return filepath.Join(base, "packs")
}

// EnsureLimboPacksDir creates the resource pack directory so users can drop
// packs in it before starting transfer mode.
func EnsureLimboPacksDir(cfg config.Config) (string, error) {
	dir := DefaultLimboPacksDir(cfg)
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

func startLimboProcess(cfg config.Config, logger *log.Logger, onLine func(string)) (*limboProcess, error) {
	path, err := limboExecutable(cfg)
	if err != nil {
		return nil, err
	}
	// The pack directory may have been changed after startup; create it before
	// limbo is launched so an explicit path never points at a missing folder.
	packsDir, err := EnsureLimboPacksDir(cfg)
	if err != nil {
		return nil, err
	}
	args := []string{
		"-listen", cfg.LimboUpstream,
		"-target", cfg.Upstream,
		"-watch-parent=true",
		"-form-title", cfg.LimboFormTitle,
		"-form-content", cfg.LimboFormContent,
		"-form-button", cfg.LimboFormButton,
	}
	if cfg.LimboPacksDir != "" {
		args = append(args, "-packs", packsDir)
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
		if _, err := goraknet.PingTimeout(cfg.LimboUpstream, 400*time.Millisecond); err == nil {
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
			return nil, fmt.Errorf("limbo 辅助进程未在 %s 响应", cfg.LimboUpstream)
		}
	}
}

func (m *Manager) ensureLimbo() error {
	m.mu.Lock()
	if m.limbo != nil {
		m.mu.Unlock()
		return nil
	}
	cfg := m.cfg
	logger := m.logger
	m.mu.Unlock()

	process, err := startLimboProcess(cfg, logger, m.recordLimboLine)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if m.limbo != nil {
		m.mu.Unlock()
		process.stop()
		return nil
	}
	m.limbo = process
	m.mu.Unlock()
	go func() {
		<-process.done
		m.mu.Lock()
		if m.limbo == process {
			m.limbo = nil
		}
		m.mu.Unlock()
	}()
	return nil
}

// recordLimboLine surfaces important limbo diagnostics in the console without
// forwarding the routine per-connection logs.
func (m *Manager) recordLimboLine(line string) {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "login"), strings.Contains(lower, "encryption"),
		strings.Contains(lower, "failed"), strings.Contains(lower, "panic"),
		strings.Contains(lower, "accept:"), strings.Contains(lower, "start game:"):
		m.Record("limbo", line)
	}
}

func (m *Manager) stopLimbo() {
	m.mu.Lock()
	process := m.limbo
	m.limbo = nil
	m.mu.Unlock()
	if process != nil {
		process.stop()
	}
}

func dialUpstream(ctx context.Context, cfg config.Config) (relay.Stream, error) {
	if cfg.ConnectionMode == config.ConnectionTransfer {
		return goraknet.DialContext(ctx, cfg.LimboUpstream)
	}
	return nemcraknet.DialContext(ctx, cfg.Upstream)
}

func upstreamAddress(cfg config.Config) string {
	if cfg.ConnectionMode == config.ConnectionTransfer {
		return cfg.LimboUpstream
	}
	return cfg.Upstream
}

func upstreamDescription(cfg config.Config) string {
	if cfg.ConnectionMode == config.ConnectionTransfer {
		return fmt.Sprintf("limbo %s → %s", cfg.LimboUpstream, cfg.Upstream)
	}
	return cfg.Upstream
}

func pingUpstream(ctx context.Context, cfg config.Config) (string, error) {
	if cfg.ConnectionMode == config.ConnectionTransfer {
		// go-raknet's PingContext may wait indefinitely on a silent endpoint.
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if _, err := goraknet.PingContext(pingCtx, cfg.LimboUpstream); err != nil {
			return "", err
		}
		return "limbo", nil
	}
	return bridge.PingServer(ctx, cfg.Upstream)
}

func (p *limboProcess) stop() {
	if p == nil {
		return
	}
	p.once.Do(func() {
		if p.stdin != nil {
			_ = p.stdin.Close()
		}
		if p.cmd.Process != nil {
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
