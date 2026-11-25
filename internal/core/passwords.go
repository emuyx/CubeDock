package core

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/emuyx/cubedock/internal/secret"
)

const accountPasswordFile = "account-passwords.dat"

// passwordKey prefers the stable NetEase UID. Email is the fallback for
// accounts imported before their UID was resolved.
func passwordKey(record accountRecord) string {
	if record.UID != "" {
		return "uid:" + record.UID
	}
	if record.Email != "" {
		return "email:" + record.Email
	}
	return ""
}

func (m *Manager) loadPasswordsLocked() (map[string]string, error) {
	raw, err := secret.Load(filepath.Join(m.dir, accountPasswordFile))
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return map[string]string{}, nil
	}
	var stored map[string]string
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return nil, fmt.Errorf("读取本地保存密码失败: %w", err)
	}
	if stored == nil {
		stored = map[string]string{}
	}
	return stored, nil
}

func (m *Manager) savePasswordsLocked(stored map[string]string) error {
	raw, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return secret.Save(filepath.Join(m.dir, accountPasswordFile), string(append(raw, '\n')))
}

// SaveAccountPassword stores or clears the optional local password for one
// account. The file is DPAPI-protected on Windows and 0600 elsewhere.
func (m *Manager) SaveAccountPassword(index int, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.accounts) {
		return fmt.Errorf("账号序号无效")
	}
	key := passwordKey(m.accounts[index])
	if key == "" {
		return fmt.Errorf("该账号没有 UID 或邮箱，无法保存密码")
	}
	stored, err := m.loadPasswordsLocked()
	if err != nil {
		return err
	}
	if password == "" {
		delete(stored, key)
	} else {
		stored[key] = password
	}
	return m.savePasswordsLocked(stored)
}

// AccountPassword returns the locally stored password for one account.
func (m *Manager) AccountPassword(index int) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.accounts) {
		return "", false, fmt.Errorf("账号序号无效")
	}
	key := passwordKey(m.accounts[index])
	if key == "" {
		return "", false, nil
	}
	stored, err := m.loadPasswordsLocked()
	if err != nil {
		return "", false, err
	}
	password, ok := stored[key]
	return password, ok, nil
}

func (m *Manager) deleteAccountPasswordLocked(record accountRecord) {
	key := passwordKey(record)
	if key == "" {
		return
	}
	stored, err := m.loadPasswordsLocked()
	if err != nil || len(stored) == 0 {
		return
	}
	if _, ok := stored[key]; !ok {
		return
	}
	delete(stored, key)
	_ = m.savePasswordsLocked(stored)
}
