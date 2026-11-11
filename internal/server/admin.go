package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

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

// Admin owns the console credentials and live session state. The password hash
// lives in admin.json with 0600 permissions; sessions are in memory only.
type Admin struct {
	filePath string

	passwordMu      sync.RWMutex
	username        string
	password        []byte
	setupRequired   bool
	passwordVersion uint64

	sessionMu sync.Mutex
	sessions  map[string]time.Time

	attemptMu sync.Mutex
	attempts  map[string]*attemptWindow

	loginGate chan struct{}
}

// LoadAdmin loads admin.json. A missing file puts the console into first-run
// setup mode instead of generating a password: the operator creates the
// account from the local console. There is deliberately no CLI/env reset.
func LoadAdmin(dir string) (*Admin, error) {
	path := filepath.Join(dir, "admin.json")
	stored, exists, err := readAdminFile(path)
	if err != nil {
		return nil, err
	}
	admin := &Admin{
		filePath:  path,
		username:  "admin",
		sessions:  make(map[string]time.Time),
		attempts:  make(map[string]*attemptWindow),
		loginGate: make(chan struct{}, 2),
	}
	if !exists {
		admin.setupRequired = true
		return admin, nil
	}
	username := strings.TrimSpace(stored.Username)
	if username == "" {
		return nil, errors.New("管理员凭据文件缺少账号名；请备份后删除 admin.json，重启并重新初始化")
	}
	if _, err := bcrypt.Cost([]byte(stored.Hash)); err != nil {
		return nil, errors.New("管理员密码哈希无效；请备份后删除 admin.json，重启并重新初始化")
	}
	admin.username = username
	admin.password = []byte(stored.Hash)
	return admin, nil
}

// NeedsSetup reports whether the operator still has to create the account.
func (a *Admin) NeedsSetup() bool {
	a.passwordMu.RLock()
	defer a.passwordMu.RUnlock()
	return a.setupRequired
}

// Username returns the configured admin name.
func (a *Admin) Username() string {
	a.passwordMu.RLock()
	defer a.passwordMu.RUnlock()
	return a.username
}

// CompleteSetup creates the first admin account. It only works while setup is
// required, so it cannot be used to replace an existing password.
func (a *Admin) CompleteSetup(username, password string) error {
	username = strings.TrimSpace(username)
	if err := validateAdminUsername(username); err != nil {
		return err
	}
	if err := validateAdminPassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	a.passwordMu.Lock()
	defer a.passwordMu.Unlock()
	if !a.setupRequired {
		return errors.New("管理员账户已经初始化")
	}
	if err := saveAdminFile(a.filePath, adminFile{Username: username, Hash: string(hash)}); err != nil {
		return err
	}
	a.username = username
	a.password = hash
	a.setupRequired = false
	a.passwordVersion++
	a.sessionMu.Lock()
	a.sessions = make(map[string]time.Time)
	a.sessionMu.Unlock()
	return nil
}

func validateAdminUsername(username string) error {
	if username == "" {
		return errors.New("请填写管理员账号")
	}
	if len([]rune(username)) > 64 {
		return errors.New("管理员账号最多 64 个字符")
	}
	for _, r := range username {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return errors.New("管理员账号不能包含空格或控制字符")
		}
	}
	return nil
}

func validateAdminPassword(password string) error {
	if len([]byte(password)) < 8 || len([]byte(password)) > 72 {
		return errors.New("管理员密码必须为 8–72 字节")
	}
	return nil
}

func (a *Admin) credentials() (string, []byte, uint64) {
	a.passwordMu.RLock()
	defer a.passwordMu.RUnlock()
	return a.username, append([]byte(nil), a.password...), a.passwordVersion
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
	if err := json.Unmarshal(raw, &stored); err != nil {
		return adminFile{}, true, errors.New("管理员凭据文件损坏；请备份后删除 admin.json，重启并重新初始化")
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
