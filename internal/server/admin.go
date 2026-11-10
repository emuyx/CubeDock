package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/emuyx/cubedock/internal/fileutil"
)

const (
	adminCookieName = "cubedock_admin"
	adminSessionTTL = 12 * time.Hour
	adminMaxTries   = 5
	adminMaxSession = 256
)

type adminFile struct {
	Username string `json:"username"`
	Hash     string `json:"password_hash"`
}

type attemptWindow struct {
	count int
	start time.Time
}

// Admin owns the console password and live session state. The password hash
// lives in admin.json with 0600 permissions; sessions are in memory only.
type Admin struct {
	filePath   string
	passwordMu sync.RWMutex
	password   []byte
	// passwordVersion increments on every change so a login that validated the
	// old password cannot insert a session after ChangePassword cleared them.
	passwordVersion uint64

	sessionMu sync.Mutex
	sessions  map[string]time.Time

	attemptMu sync.Mutex
	attempts  map[string]*attemptWindow

	loginGate chan struct{}
}

// LoadAdmin loads or creates admin.json. The returned string is only non-empty
// when CubeDock generated a fresh password for one-time terminal display.
func LoadAdmin(dir, override string, reset bool) (*Admin, string, error) {
	path := filepath.Join(dir, "admin.json")
	stored, exists, err := readAdminFile(path)
	if err != nil {
		return nil, "", err
	}
	if exists && !reset {
		if stored.Username != "admin" {
			return nil, "", fmt.Errorf("管理员凭据无效，请使用 -reset-admin-password 重置")
		}
		if _, err := bcrypt.Cost([]byte(stored.Hash)); err != nil {
			return nil, "", fmt.Errorf("管理员密码哈希无效，请使用 -reset-admin-password 重置")
		}
	}

	generated := ""
	if override != "" || !exists || reset {
		password := override
		if password == "" {
			token, err := randomText(18)
			if err != nil {
				return nil, "", err
			}
			password = token
			generated = token
		}
		if len([]byte(password)) < 8 || len([]byte(password)) > 72 {
			return nil, "", fmt.Errorf("管理密码必须为 8–72 字节")
		}
		unchanged := exists && !reset &&
			bcrypt.CompareHashAndPassword([]byte(stored.Hash), []byte(password)) == nil
		if !unchanged {
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return nil, "", err
			}
			stored = adminFile{Username: "admin", Hash: string(hash)}
			if err := saveAdminFile(path, stored); err != nil {
				return nil, "", err
			}
		}
	}

	return &Admin{
		filePath:  path,
		password:  []byte(stored.Hash),
		sessions:  make(map[string]time.Time),
		attempts:  make(map[string]*attemptWindow),
		loginGate: make(chan struct{}, 2),
	}, generated, nil
}

func readAdminFile(path string) (adminFile, bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return adminFile{}, false, nil
	}
	if err != nil {
		return adminFile{}, false, fmt.Errorf("读取管理员凭据失败: %w", err)
	}
	var stored adminFile
	if json.Unmarshal(raw, &stored) != nil {
		// Leave validation to LoadAdmin so -reset-admin-password can recover
		// from a corrupt file.
		return adminFile{}, true, nil
	}
	return stored, true, nil
}

func saveAdminFile(path string, stored adminFile) error {
	raw, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	if err := fileutil.Replace(path, append(raw, '\n'), 0600); err != nil {
		return fmt.Errorf("保存管理员凭据失败: %w", err)
	}
	return nil
}

func randomText(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
