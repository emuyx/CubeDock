package core

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/emuyx/cubedock/internal/fileutil"
)

// AccountStatus is the non-secret account metadata exposed to the console.
// The session token itself lives in the protected room-token file.
type AccountStatus struct {
	Index        int    `json:"index"`
	Email        string `json:"email"`
	Label        string `json:"label,omitempty"`
	Name         string `json:"name,omitempty"`
	UID          string `json:"uid,omitempty"`
	Level        int    `json:"level,omitempty"`
	Platform     string `json:"platform,omitempty"`   // interop = 基岩互通版会话
	ExpiresAt    string `json:"expires_at,omitempty"` // access_token 到期时间
	ProfileError string `json:"profile_error,omitempty"`
	VerifiedAt   string `json:"verified_at,omitempty"`
}

type accountRecord struct {
	Email        string `json:"email,omitempty"`
	Label        string `json:"label,omitempty"`
	Name         string `json:"name,omitempty"`
	UID          string `json:"uid,omitempty"`
	Level        int    `json:"level,omitempty"`
	Platform     string `json:"platform,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	ProfileError string `json:"profile_error,omitempty"`
	VerifiedAt   string `json:"verified_at,omitempty"`
}

// AccountMeta is supplied when a session is added or replaced.
type AccountMeta struct {
	Email        string
	Label        string
	Name         string
	UID          string
	Level        int
	Platform     string
	ExpiresAt    string
	ProfileError string
	Verified     string
}

func loadAccounts(path string, tokenCount int) ([]accountRecord, error) {
	var records []accountRecord
	changed := false
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if json.Unmarshal(raw, &records) != nil {
			records = nil
			changed = true
		}
	case os.IsNotExist(err):
		changed = true
	default:
		return nil, err
	}
	for len(records) < tokenCount {
		records = append(records, accountRecord{Label: "未命名"})
		changed = true
	}
	if len(records) > tokenCount {
		records = records[:tokenCount]
		changed = true
	}
	// Persist the normalised shape so the console never has to guess whether
	// metadata exists and a corrupt file is replaced instead of reused.
	if changed {
		if saveErr := saveAccounts(path, records); saveErr != nil {
			return nil, saveErr
		}
	}
	return records, nil
}

func saveAccounts(path string, records []accountRecord) error {
	raw, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.Replace(filepath.Clean(path), append(raw, '\n'), 0600)
}

func (m *Manager) accountStatusesLocked() []AccountStatus {
	out := make([]AccountStatus, len(m.accounts))
	for index, record := range m.accounts {
		out[index] = AccountStatus{
			Index: index, Email: record.Email, Label: record.Label,
			Name: record.Name, UID: record.UID, Level: record.Level,
			Platform: record.Platform, ExpiresAt: record.ExpiresAt,
			ProfileError: record.ProfileError, VerifiedAt: record.VerifiedAt,
		}
	}
	return out
}

// Accounts returns a copy of the console-visible account list.
func (m *Manager) Accounts() []AccountStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.accountStatusesLocked()
}
