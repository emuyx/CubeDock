package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ChangePassword verifies the current password, writes the new bcrypt hash and
// drops every existing console session.
func (a *Admin) ChangePassword(current, next string) error {
	if err := validateAdminPassword(next); err != nil {
		return fmt.Errorf("新密码必须为 8–72 字节")
	}
	if current == "" {
		return fmt.Errorf("请输入当前密码")
	}
	if bcrypt.CompareHashAndPassword(a.currentHash(), []byte(current)) != nil {
		return fmt.Errorf("当前密码错误")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	username := a.Username()
	if err := saveAdminFile(a.filePath, adminFile{Username: username, Hash: string(hash)}); err != nil {
		return err
	}
	a.passwordMu.Lock()
	a.password = hash
	a.passwordVersion++
	a.passwordMu.Unlock()
	a.sessionMu.Lock()
	a.sessions = make(map[string]time.Time)
	a.sessionMu.Unlock()
	return nil
}

func (a *Admin) login(w http.ResponseWriter, r *http.Request) {
	if a.NeedsSetup() {
		http.Error(w, "请先在控制台创建管理员账户", http.StatusConflict)
		return
	}
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
	case a.loginGate <- struct{}{}:
		defer func() { <-a.loginGate }()
	default:
		http.Error(w, "登录服务繁忙，请稍后重试", http.StatusTooManyRequests)
		return
	}
	username, hash, version := a.credentials()
	if strings.TrimSpace(input.Username) != username ||
		bcrypt.CompareHashAndPassword(hash, []byte(input.Password)) != nil {
		http.Error(w, "账号或密码错误", http.StatusUnauthorized)
		return
	}
	if err := a.issueSession(w, r, version); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// issueSession creates a new session token and stores it in the in-memory
// session map. version prevents a login that validated an old password from
// surviving a concurrent password change.
func (a *Admin) issueSession(w http.ResponseWriter, r *http.Request, version uint64) error {
	token, err := randomText(32)
	if err != nil {
		return errors.New("创建登录会话失败")
	}
	now := time.Now()
	a.sessionMu.Lock()
	a.passwordMu.RLock()
	stale := a.passwordVersion != version
	a.passwordMu.RUnlock()
	if stale {
		a.sessionMu.Unlock()
		return errors.New("管理员密码已更改，请重新登录")
	}
	for key, expiry := range a.sessions {
		if !now.Before(expiry) {
			delete(a.sessions, key)
		}
	}
	if len(a.sessions) >= adminMaxSession {
		var oldestKey string
		var oldestExpiry time.Time
		for key, expiry := range a.sessions {
			if oldestKey == "" || expiry.Before(oldestExpiry) {
				oldestKey, oldestExpiry = key, expiry
			}
		}
		delete(a.sessions, oldestKey)
	}
	a.sessions[sessionKey(token)] = now.Add(adminSessionTTL)
	a.sessionMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name: adminCookieName, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil,
		MaxAge: int(adminSessionTTL.Seconds()),
	})
	return nil
}

func (a *Admin) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(adminCookieName); err == nil {
		a.sessionMu.Lock()
		delete(a.sessions, sessionKey(cookie.Value))
		a.sessionMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name: adminCookieName, Path: "/", MaxAge: -1, HttpOnly: true,
		SameSite: http.SameSiteStrictMode, Secure: r.TLS != nil,
	})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"ok":true}`))
}
