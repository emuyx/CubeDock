package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/emuyx/cubedock/internal/fileutil"
)

const adminCookieName = "cubedock_admin"
const adminSessionTTL = 12 * time.Hour

type adminCredential struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
}

type loginAttempts struct {
	count int
	since time.Time
}

type Admin struct {
	path     string
	hash     []byte
	hashMu   sync.RWMutex
	mu       sync.Mutex
	sessions map[[32]byte]time.Time
	attempts map[string]loginAttempts
	verify   chan struct{}
}

// Only a newly generated password is returned for one-time terminal display.
// An environment-provided password is never echoed or saved in plaintext.
func LoadAdmin(dir, override string, reset bool) (*Admin, string, error) {
	path := filepath.Join(dir, "admin.json")
	var credential adminCredential
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, "", fmt.Errorf("读取管理员凭据失败: %w", err)
	}
	exists := err == nil
	if exists && !reset {
		if json.Unmarshal(raw, &credential) != nil || credential.Username != "admin" {
			return nil, "", fmt.Errorf("管理员凭据无效，请使用 -reset-admin-password 重置")
		}
		if _, err := bcrypt.Cost([]byte(credential.PasswordHash)); err != nil {
			return nil, "", fmt.Errorf("管理员密码哈希无效，请使用 -reset-admin-password 重置")
		}
	}
	initialPassword := ""
	if override != "" || !exists || reset {
		password := override
		if password == "" {
			var random [18]byte
			if _, err := rand.Read(random[:]); err != nil {
				return nil, "", err
			}
			password = base64.RawURLEncoding.EncodeToString(random[:])
			initialPassword = password
		}
		if len(password) < 8 || len(password) > 72 {
			return nil, "", fmt.Errorf("管理密码必须为 8–72 字节")
		}
		if !exists || reset || bcrypt.CompareHashAndPassword([]byte(credential.PasswordHash), []byte(password)) != nil {
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return nil, "", err
			}
			credential = adminCredential{Username: "admin", PasswordHash: string(hash)}
			raw, err := json.MarshalIndent(credential, "", "  ")
			if err != nil {
				return nil, "", err
			}
			if err := fileutil.Replace(path, append(raw, '\n'), 0600); err != nil {
				return nil, "", fmt.Errorf("保存管理员凭据失败: %w", err)
			}
		}
	}
	return &Admin{
		path: path, hash: []byte(credential.PasswordHash), sessions: make(map[[32]byte]time.Time),
		attempts: make(map[string]loginAttempts), verify: make(chan struct{}, 2),
	}, initialPassword, nil
}

func (a *Admin) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(adminCookieName)
	if err != nil || len(cookie.Value) != 43 {
		return false
	}
	key := sha256.Sum256([]byte(cookie.Value))
	a.mu.Lock()
	defer a.mu.Unlock()
	expires, exists := a.sessions[key]
	if exists && !time.Now().Before(expires) {
		delete(a.sessions, key)
		return false
	}
	return exists
}

func (a *Admin) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/login", r.URL.Path == "/favicon.svg", strings.HasPrefix(r.URL.Path, "/assets/"):
			next.ServeHTTP(w, r)
			return
		case r.URL.Path == "/login":
			if a.authenticated(r) {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if !a.authenticated(r) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.Error(w, "请先登录管理页面", http.StatusUnauthorized)
			} else {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *Admin) allowAttempt(r *http.Request) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, entry := range a.attempts {
		if now.Sub(entry.since) >= time.Minute {
			delete(a.attempts, key)
		}
	}
	entry, exists := a.attempts[ip]
	if entry.count >= 5 || (!exists && len(a.attempts) >= 1024) {
		return false
	}
	if !exists {
		entry.since = now
	}
	entry.count++
	a.attempts[ip] = entry
	return true
}

// changePassword verifies the current password, stores the new bcrypt hash and
// invalidates every existing console session.
func (a *Admin) ChangePassword(current, next string) error {
	if len([]byte(next)) < 8 || len([]byte(next)) > 72 {
		return fmt.Errorf("新密码必须为 8–72 字节")
	}
	if current == "" {
		return fmt.Errorf("请输入当前密码")
	}
	a.hashMu.RLock()
	hash := append([]byte(nil), a.hash...)
	a.hashMu.RUnlock()
	if bcrypt.CompareHashAndPassword(hash, []byte(current)) != nil {
		return fmt.Errorf("当前密码错误")
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	credential := adminCredential{Username: "admin", PasswordHash: string(newHash)}
	raw, err := json.MarshalIndent(credential, "", "  ")
	if err != nil {
		return err
	}
	if err := fileutil.Replace(a.path, append(raw, '\n'), 0600); err != nil {
		return fmt.Errorf("保存管理员凭据失败: %w", err)
	}
	a.hashMu.Lock()
	a.hash = newHash
	a.hashMu.Unlock()
	a.mu.Lock()
	a.sessions = make(map[[32]byte]time.Time)
	a.mu.Unlock()
	return nil
}

func (a *Admin) login(w http.ResponseWriter, r *http.Request) {
	if !a.allowAttempt(r) {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "登录尝试过于频繁，请一分钟后再试", http.StatusTooManyRequests)
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || len(input.Password) > 72 {
		http.Error(w, "登录数据无效", http.StatusBadRequest)
		return
	}
	select {
	case a.verify <- struct{}{}:
		defer func() { <-a.verify }()
	default:
		http.Error(w, "登录服务繁忙，请稍后重试", http.StatusTooManyRequests)
		return
	}
	a.hashMu.RLock()
	hash := append([]byte(nil), a.hash...)
	a.hashMu.RUnlock()
	passwordErr := bcrypt.CompareHashAndPassword(hash, []byte(input.Password))
	if passwordErr != nil || input.Username != "admin" {
		http.Error(w, "账号或密码错误", http.StatusUnauthorized)
		return
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		http.Error(w, "创建登录会话失败", http.StatusInternalServerError)
		return
	}
	token := base64.RawURLEncoding.EncodeToString(random[:])
	now := time.Now()
	a.mu.Lock()
	for key, expiry := range a.sessions {
		if !now.Before(expiry) {
			delete(a.sessions, key)
		}
	}
	if len(a.sessions) >= 256 {
		var oldest [32]byte
		var earliest time.Time
		for key, expiry := range a.sessions {
			if earliest.IsZero() || expiry.Before(earliest) {
				oldest, earliest = key, expiry
			}
		}
		delete(a.sessions, oldest)
	}
	a.sessions[sha256.Sum256([]byte(token))] = now.Add(adminSessionTTL)
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: adminCookieName, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil, MaxAge: int(adminSessionTTL.Seconds())})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (a *Admin) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(adminCookieName); err == nil {
		a.mu.Lock()
		delete(a.sessions, sha256.Sum256([]byte(cookie.Value)))
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: adminCookieName, Path: "/", MaxAge: -1, HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"ok":true}`))
}
