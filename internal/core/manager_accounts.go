// Package core owns CubeDock's room lifecycle, account list and event log. It
// has no HTTP dependencies so the console is only one possible front end.
package core

import (
	"fmt"
	"github.com/emuyx/cubedock/internal/secret"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Token returns the stored session for one room.
func (m *Manager) Token(index int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.tokens) {
		return "", fmt.Errorf("账号序号无效")
	}
	return m.tokens[index], nil
}

// AddAccount appends a session and its metadata, returning the room index.
func (m *Manager) AddAccount(token string, meta AccountMeta) (int, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return -1, fmt.Errorf("账号会话为空")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return -1, fmt.Errorf("请先停止房间，再添加账号")
	}
	if len(m.tokens) >= MaxRooms {
		return -1, fmt.Errorf("最多同时开 %d 个房间", MaxRooms)
	}
	for _, existing := range m.tokens {
		if existing == token {
			// One account cannot keep two rooms alive; the second login drops the first.
			return -1, fmt.Errorf("这个网易账号已经添加过了，请先移除或更换账号")
		}
	}
	nextTokens := append(append([]string(nil), m.tokens...), token)
	nextAccounts := append(append([]accountRecord(nil), m.accounts...), recordFromMeta(meta))
	if err := m.persistIdentityLocked(nextTokens, nextAccounts); err != nil {
		return -1, err
	}
	m.tokens, m.accounts = nextTokens, nextAccounts
	return len(nextTokens) - 1, nil
}

// ReplaceAccount swaps the session and metadata of an existing room.
func (m *Manager) ReplaceAccount(index int, token string, meta AccountMeta) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("账号会话为空")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return fmt.Errorf("请先停止房间，再更新账号")
	}
	if index < 0 || index >= len(m.tokens) {
		return fmt.Errorf("账号序号无效")
	}
	for i, existing := range m.tokens {
		if i != index && existing == token {
			return fmt.Errorf("这个网易账号已经用于房间 %d，不能重复使用", i+1)
		}
	}
	nextTokens := append([]string(nil), m.tokens...)
	nextTokens[index] = token
	nextAccounts := append([]accountRecord(nil), m.accounts...)
	nextAccounts[index] = recordFromMeta(meta)
	if err := m.persistIdentityLocked(nextTokens, nextAccounts); err != nil {
		return err
	}
	m.tokens, m.accounts = nextTokens, nextAccounts
	return nil
}

// RemoveAccount removes a room's session and metadata.
func (m *Manager) RemoveAccount(index int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return fmt.Errorf("请先停止房间，再移除账号")
	}
	if index < 0 || index >= len(m.tokens) {
		return fmt.Errorf("账号序号无效")
	}
	nextTokens := append([]string(nil), m.tokens[:index]...)
	nextTokens = append(nextTokens, m.tokens[index+1:]...)
	nextAccounts := append([]accountRecord(nil), m.accounts[:index]...)
	nextAccounts = append(nextAccounts, m.accounts[index+1:]...)
	if err := m.persistIdentityLocked(nextTokens, nextAccounts); err != nil {
		return err
	}
	m.tokens, m.accounts = nextTokens, nextAccounts
	return nil
}

// UpdateAccountProfile updates the resolved game identity or its error.
func (m *Manager) UpdateAccountProfile(index int, name, uid string, level int, profileErr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.accounts) {
		return fmt.Errorf("账号序号无效")
	}
	next := append([]accountRecord(nil), m.accounts...)
	next[index].Name = name
	next[index].UID = uid
	next[index].Level = level
	next[index].ProfileError = profileErr
	if profileErr == "" && name != "" {
		next[index].VerifiedAt = time.Now().Format(time.RFC3339)
	}
	if err := saveAccounts(filepath.Join(m.dir, accountFile), next); err != nil {
		return err
	}
	m.accounts = next
	return nil
}

func recordFromMeta(meta AccountMeta) accountRecord {
	record := accountRecord{
		Email: meta.Email, Label: meta.Label, Name: meta.Name, UID: meta.UID,
		Level: meta.Level, Platform: meta.Platform, ExpiresAt: meta.ExpiresAt,
		ProfileError: meta.ProfileError, VerifiedAt: meta.Verified,
	}
	if record.VerifiedAt == "" && record.Name != "" {
		record.VerifiedAt = time.Now().Format(time.RFC3339)
	}
	return record
}

// persistIdentityLocked writes both the protected token list and the account
// metadata, restoring the previous token file if the second write fails.
func (m *Manager) persistIdentityLocked(tokens []string, accounts []accountRecord) error {
	tokenPath := filepath.Join(m.dir, tokenFile)
	previous := strings.Join(m.tokens, "\n")
	if err := saveTokens(tokenPath, tokens); err != nil {
		return err
	}
	if err := saveAccounts(filepath.Join(m.dir, accountFile), accounts); err != nil {
		_ = saveTokens(tokenPath, splitTokens(previous))
		return fmt.Errorf("保存账号信息失败: %w", err)
	}
	return nil
}

func saveTokens(path string, tokens []string) error {
	if len(tokens) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return secret.Save(path, strings.Join(tokens, "\n"))
}

func splitTokens(raw string) []string { return strings.Fields(raw) }
