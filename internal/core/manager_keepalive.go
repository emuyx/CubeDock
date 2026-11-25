package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/netease"
)

// keepAliveLoop refreshes idle account profiles so sessions and the console's
// name/avatar/expiry information do not go stale while no room is running.
// It deliberately skips every account while any room is active: a second PE
// authentication can disturb an online room, and the running account already
// re-authenticates on its retry loop.
func (m *Manager) keepAliveLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	last := time.Now()
	for {
		select {
		case <-m.exit:
			return
		case now := <-ticker.C:
			m.mu.Lock()
			cfg := m.cfg
			busy := len(m.runtimes) > 0
			m.mu.Unlock()
			if busy || !cfg.KeepAlive {
				continue
			}
			minutes := cfg.KeepAliveMinutes
			if minutes < 5 {
				minutes = 30
			}
			if now.Sub(last) < time.Duration(minutes)*time.Minute {
				continue
			}
			last = now
			m.keepAliveIdleAccounts()
		}
	}
}

type keepAliveTarget struct {
	index  int
	token  string
	method netease.AuthMethod
}

func (m *Manager) keepAliveIdleAccounts() {
	m.mu.Lock()
	if len(m.runtimes) > 0 || !m.cfg.KeepAlive {
		m.mu.Unlock()
		return
	}
	now := time.Now()
	targets := make([]keepAliveTarget, 0, len(m.accounts))
	for index, account := range m.accounts {
		if index >= len(m.tokens) || m.tokens[index] == "" {
			continue
		}
		method, err := netease.ParseAuthMethod(account.AuthMethod)
		if err != nil {
			continue
		}
		if account.ExpiresAt != "" {
			expires, parseErr := time.Parse(time.RFC3339, account.ExpiresAt)
			if parseErr == nil && !expires.After(now) {
				continue
			}
		}
		targets = append(targets, keepAliveTarget{index: index, token: m.tokens[index], method: method})
	}
	m.mu.Unlock()

	if len(targets) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	updated, failed := 0, 0
	for _, target := range targets {
		select {
		case <-m.exit:
			return
		case <-ctx.Done():
			return
		default:
		}
		m.mu.Lock()
		busy := len(m.runtimes) > 0
		m.mu.Unlock()
		if busy {
			return
		}
		authCtx, authCancel := context.WithTimeout(ctx, 45*time.Second)
		profile, err := auth.LookupProfileFor(authCtx, target.method, target.token)
		authCancel()
		if err != nil && !errors.Is(err, netease.ErrProfileNameMissing) {
			failed++
		} else {
			profileErr := ""
			if errors.Is(err, netease.ErrProfileNameMissing) {
				profileErr = netease.ErrProfileNameMissing.Error()
			}
			if updateErr := m.UpdateAccountProfileForToken(
				target.index, target.token, profile.Name, profile.UID, profile.Level, profile.AvatarURL, profileErr,
			); updateErr != nil {
				failed++
			} else {
				updated++
			}
		}
		timer := time.NewTimer(5 * time.Second)
		select {
		case <-m.exit:
			timer.Stop()
			return
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
	if updated > 0 || failed > 0 {
		m.Record("账号", fmt.Sprintf("空闲账号保活：刷新 %d 个，失败 %d 个", updated, failed))
	}
}
