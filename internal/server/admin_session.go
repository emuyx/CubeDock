package server

import (
	"crypto/sha256"
	"encoding/base64"
	"net"
	"net/http"
	"strings"
	"time"
)

func sessionKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (a *Admin) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(adminCookieName)
	if err != nil || len(cookie.Value) != 43 {
		return false
	}
	key := sessionKey(cookie.Value)
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	expiry, ok := a.sessions[key]
	if !ok {
		return false
	}
	if !time.Now().Before(expiry) {
		delete(a.sessions, key)
		return false
	}
	return true
}

// Protect enforces the console session cookie. Login, static assets and the
// favicon stay public; an already-authenticated /login request is redirected.
func (a *Admin) Protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/api/login" || path == "/favicon.svg" || strings.HasPrefix(path, "/assets/") {
			// Static assets are public, but the embedded file server would
			// otherwise list the directory for a trailing slash.
			if strings.HasSuffix(path, "/") {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if path == "/login" {
			if a.authenticated(r) {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if a.authenticated(r) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(path, "/api/") {
			http.Error(w, "请先登录管理页面", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (a *Admin) allowAttempt(r *http.Request) bool {
	ip := remoteIP(r)
	now := time.Now()
	a.attemptMu.Lock()
	defer a.attemptMu.Unlock()
	for address, window := range a.attempts {
		if now.Sub(window.start) >= time.Minute {
			delete(a.attempts, address)
		}
	}
	window := a.attempts[ip]
	if window == nil {
		if len(a.attempts) >= 1024 {
			return false
		}
		window = &attemptWindow{start: now}
		a.attempts[ip] = window
	}
	if window.count >= adminMaxTries {
		return false
	}
	window.count++
	return true
}

func (a *Admin) currentHash() []byte {
	a.passwordMu.RLock()
	defer a.passwordMu.RUnlock()
	return append([]byte(nil), a.password...)
}

// passwordSnapshot returns the current hash and version together, so a login
// can detect a password change that happened while bcrypt was running.
func (a *Admin) passwordSnapshot() ([]byte, uint64) {
	a.passwordMu.RLock()
	defer a.passwordMu.RUnlock()
	return append([]byte(nil), a.password...), a.passwordVersion
}
