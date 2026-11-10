// Package core owns CubeDock's room lifecycle, account list and event log. It
// has no HTTP dependencies so the console is only one possible front end.
package core

import (
	"context"
	"fmt"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/session"
	"strings"
	"sync"
	"time"
)

// Start launches one room goroutine per account. Calling Start while running is
// a no-op.
func (m *Manager) Start() error {
	// Configure and SetDebug write cfg under m.mu; snapshot it once so the
	// pre-flight checks below never read it unsynchronised.
	m.mu.Lock()
	cfg := m.cfg
	m.mu.Unlock()
	if cfg.ConnectionMode == config.ConnectionTransfer {
		m.mu.Lock()
		running := m.cancel != nil
		hasTokens := len(m.tokens) > 0
		m.mu.Unlock()
		if cfg.LimboAutoPullPacks && !running && hasTokens {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			output, pullErr := m.pullResourcePacks(ctx)
			cancel()
			dir, dirErr := EnsureLimboPacksDir(cfg)
			switch {
			case pullErr != nil:
				m.Record("资源包", "自动拉取后端资源包失败："+pullErr.Error())
			default:
				m.Record("资源包", "已从后端服拉取资源包")
				if output != "" && m.logger != nil {
					m.logger.Printf("[packs] %s", output)
				}
			}
			if dirErr == nil {
				removed, warnings, dedupeErr := dedupeResourcePacks(dir)
				if dedupeErr != nil {
					m.Record("资源包", "清理重复资源包失败："+dedupeErr.Error())
				} else if len(removed) > 0 {
					m.Record("资源包", fmt.Sprintf("已删除 %d 个重复资源包：%s",
						len(removed), strings.Join(removed, ", ")))
				}
				for _, warning := range warnings {
					m.Record("资源包", warning)
				}
			}
		}
		if err := m.ensureLimbo(); err != nil {
			return err
		}
	}
	m.mu.Lock()
	success := false
	defer func() {
		if !success {
			m.stopLimbo()
		}
	}()
	defer m.mu.Unlock()
	if m.cancel != nil {
		success = true
		return nil
	}
	if len(m.tokens) == 0 {
		return fmt.Errorf("请先添加网易账号")
	}
	// Acquire every account lease before starting anything, so a busy account
	// cannot leave the remaining rooms half-started.
	leases := make([]*session.Lease, 0, len(m.tokens))
	for index, token := range m.tokens {
		lease, err := m.claimSession(token)
		if err != nil {
			for _, held := range leases {
				held.Release()
			}
			if len(m.tokens) > 1 {
				return fmt.Errorf("%s%w", roomLabel(index, len(m.tokens)), err)
			}
			return err
		}
		leases = append(leases, lease)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.finished = make(chan struct{})
	m.status.Running = true
	m.status.LastError = ""
	m.status.Attempts = 0
	m.status.Rooms = make([]RoomStatus, len(m.tokens))
	for index := range m.status.Rooms {
		m.status.Rooms[index] = RoomStatus{Phase: "checking_upstream"}
		if index < len(m.accounts) {
			m.status.Rooms[index].Account = firstNonEmpty(m.accounts[index].Name, m.accounts[index].Label, m.accounts[index].Email)
		}
	}
	cfg = m.cfg
	tokens := append([]string(nil), m.tokens...)
	finished := m.finished

	var rooms sync.WaitGroup
	for index, token := range tokens {
		rooms.Add(1)
		go func(index int, token string, lease *session.Lease) {
			defer rooms.Done()
			defer lease.Release()
			m.run(ctx, index, cfg, token)
		}(index, token, leases[index])
	}
	go func() {
		rooms.Wait()
		m.updateStatus(func(status *Status) {
			status.Running = false
			status.Phase = "stopped"
			status.RoomID = 0
			status.Players = 0
			status.Account = ""
			status.ProtocolID = 0
			status.ConnectedAt = ""
			status.RetryAt = ""
			status.LastError = ""
			status.Rooms = nil
		})
		m.mu.Lock()
		m.cancel = nil
		close(finished)
		m.mu.Unlock()
		m.stopLimbo()
		m.emit("系统", "房间已停止")
	}()
	success = true
	return nil
}

// Stop asks every room to shut down.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()
	m.stopLimbo()
}

// Wait blocks until all rooms have stopped.
func (m *Manager) Wait() {
	m.mu.Lock()
	done := m.finished
	m.mu.Unlock()
	if done != nil {
		<-done
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func playerLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "未知玩家"
	}
	return name
}

func shortDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d 秒", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d 分 %d 秒", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d 时 %d 分", int(d.Hours()), int(d.Minutes())%60)
}

// roomOccupancy returns the current guest count for console messages. The host
// bot occupies one slot, so the guest capacity is maxPlayers-1.
func (m *Manager) roomOccupancy(index, maxPlayers int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.status.Rooms) {
		return ""
	}
	return fmt.Sprintf("（房间在线 %d/%d）", m.status.Rooms[index].Players, maxPlayers-1)
}

type roomLog struct {
	manager *Manager
	source  string
}

func (l roomLog) event(message string) { l.manager.emit(l.source, message) }
func (l roomLog) debug(message string) { l.manager.trace(l.source, message) }
