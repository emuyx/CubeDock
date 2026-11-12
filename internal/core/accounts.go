package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/fileutil"
	"github.com/emuyx/cubedock/internal/netease"
)

// AccountStatus is the non-secret account metadata exposed to the console.
// The session token itself lives in the protected room-token file.
type AccountStatus struct {
	Index          int    `json:"index"`
	Email          string `json:"email"`
	Label          string `json:"label,omitempty"`
	Name           string `json:"name,omitempty"`
	UID            string `json:"uid,omitempty"`
	Level          int    `json:"level,omitempty"`
	Platform       string `json:"platform,omitempty"`    // legacy label
	AuthMethod     string `json:"auth_method,omitempty"` // interop / mobile_netease / mobile_4399
	ExpiresAt      string `json:"expires_at,omitempty"`  // access_token 到期时间
	ProfileError   string `json:"profile_error,omitempty"`
	VerifiedAt     string `json:"verified_at,omitempty"`
	ProfileID      string `json:"profile_id,omitempty"`
	ProfileName    string `json:"profile_name,omitempty"`
	ConnectionMode string `json:"connection_mode,omitempty"`
}

type accountRecord struct {
	Email        string `json:"email,omitempty"`
	Label        string `json:"label,omitempty"`
	Name         string `json:"name,omitempty"`
	UID          string `json:"uid,omitempty"`
	Level        int    `json:"level,omitempty"`
	Platform     string `json:"platform,omitempty"`
	AuthMethod   string `json:"auth_method,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	ProfileError string `json:"profile_error,omitempty"`
	VerifiedAt   string `json:"verified_at,omitempty"`
	ProfileID    string `json:"profile_id,omitempty"`
}

// AccountMeta is supplied when a session is added or replaced.
type AccountMeta struct {
	Email        string
	Label        string
	Name         string
	UID          string
	Level        int
	Platform     string
	AuthMethod   string
	ExpiresAt    string
	ProfileError string
	Verified     string
	ProfileID    string
}

func loadAccounts(path string, tokenCount int, defaultProfileID string) ([]accountRecord, error) {
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
	for index := range records {
		if records[index].ProfileID == "" {
			records[index].ProfileID = defaultProfileID
			changed = true
		}
		if records[index].AuthMethod == "" {
			records[index].AuthMethod = string(netease.AuthInterop)
			changed = true
		}
	}
	for len(records) < tokenCount {
		records = append(records, accountRecord{Label: "未命名", ProfileID: defaultProfileID})
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
		status := AccountStatus{
			Index: index, Email: record.Email, Label: record.Label,
			Name: record.Name, UID: record.UID, Level: record.Level,
			Platform: record.Platform, AuthMethod: record.AuthMethod,
			ExpiresAt:    record.ExpiresAt,
			ProfileError: record.ProfileError, VerifiedAt: record.VerifiedAt,
			ProfileID: record.ProfileID,
		}
		if status.ProfileID == "" {
			status.ProfileID = m.cfg.DefaultProfileID()
		}
		if profile, ok := m.cfg.Profile(status.ProfileID); ok {
			status.ProfileName = profile.Name
			status.ConnectionMode = profile.ConnectionMode
		}
		out[index] = status
	}
	return out
}

// AccountProfiles resolves the profile selected by every account. It is used
// by the one-shot `-check` command, which runs before the Manager exists.
func AccountProfiles(dir string, cfg config.Config, tokenCount int) ([]config.Profile, error) {
	records, err := loadAccounts(filepath.Join(dir, accountFile), tokenCount, cfg.DefaultProfileID())
	if err != nil {
		return nil, err
	}
	profiles := make([]config.Profile, len(records))
	for index, record := range records {
		id := record.ProfileID
		if id == "" {
			id = cfg.DefaultProfileID()
		}
		profile, ok := cfg.Profile(id)
		if !ok {
			return nil, fmt.Errorf("房间 %d 选择的配置方案 %q 不存在", index+1, id)
		}
		profiles[index] = profile
	}
	return profiles, nil
}

// Accounts returns a copy of the console-visible account list.
func (m *Manager) Accounts() []AccountStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.accountStatusesLocked()
}
