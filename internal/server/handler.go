// Package server adapts the CubeDock core to the browser console.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/console"
	"github.com/emuyx/cubedock/internal/core"
	"github.com/emuyx/cubedock/internal/interoplogin"
	"github.com/emuyx/cubedock/internal/redact"
	"github.com/emuyx/cubedock/internal/webui"
)

type consoleHandler struct {
	manager *core.Manager
	admin   *Admin
	interop *interoplogin.Service
	runtime RuntimeInfo
}

// NewHandler returns the complete console, including static assets and the
// admin session middleware.
func NewHandler(manager *core.Manager, admin *Admin, runtime RuntimeInfo) http.Handler {
	if admin == nil {
		panic("console authentication is required")
	}
	h := &consoleHandler{
		manager: manager,
		admin:   admin,
		interop: interoplogin.New(manager.Dir()),
		runtime: runtime.withDefaults(),
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /login", h.loginPage)
	mux.HandleFunc("POST /api/login", admin.login)
	mux.HandleFunc("POST /api/logout", admin.logout)
	mux.HandleFunc("GET /api/setup/status", h.setupStatus)
	mux.HandleFunc("POST /api/setup", h.completeSetup)
	mux.HandleFunc("POST /api/admin/password", h.changePassword)
	mux.HandleFunc("GET /api/status", h.status)
	mux.HandleFunc("GET /api/players", h.players)
	mux.HandleFunc("GET /api/room/members", h.roomMembers)
	mux.HandleFunc("POST /api/room/kick-member", h.kickRoomMember)
	mux.HandleFunc("POST /api/player/kick", h.kickPlayer)
	mux.HandleFunc("POST /api/player/room-kick", h.roomKickPlayer)
	mux.HandleFunc("POST /api/player/transfer", h.transferPlayer)
	mux.HandleFunc("GET /api/friends", h.friendRequests)
	mux.HandleFunc("POST /api/friend/accept", h.acceptFriendRequest)
	mux.HandleFunc("GET /api/diagnostics", h.diagnostics)
	mux.HandleFunc("GET /api/backup", h.backup)
	mux.HandleFunc("POST /api/restore", h.restore)
	mux.HandleFunc("GET /api/packs", h.resourcePacks)
	mux.HandleFunc("POST /api/packs/refresh", h.refreshResourcePacks)
	mux.HandleFunc("POST /api/start", h.start)
	mux.HandleFunc("POST /api/stop", h.stop)
	mux.HandleFunc("POST /api/room/start", h.startRoom)
	mux.HandleFunc("POST /api/room/stop", h.stopRoom)
	mux.HandleFunc("POST /api/room/restart", h.restartRoom)
	mux.HandleFunc("POST /api/debug", h.setDebug)
	mux.HandleFunc("POST /api/console", h.runConsole)
	mux.HandleFunc("POST /api/exit", h.exit)
	mux.HandleFunc("POST /api/config", h.saveConfig)
	mux.HandleFunc("POST /api/session/import", h.importSession)
	mux.HandleFunc("POST /api/session/interop-login", h.interopLogin)
	mux.HandleFunc("POST /api/account/remove", h.removeAccount)
	mux.HandleFunc("POST /api/account/profile", h.refreshProfile)
	mux.HandleFunc("POST /api/account/assign-profile", h.assignProfile)
	mux.Handle("GET /", http.FileServer(http.FS(webui.Root())))

	port := manager.Snapshot().Config.WebPort
	allowedHosts := console.Hosts(port, console.IPv4Addresses())
	protected := admin.Protect(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHosts[r.Host] {
			http.Error(w, "控制台访问地址无效，请使用本机或终端显示的 IP 地址", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; base-uri 'none'; form-action 'self'; object-src 'none'; "+
				"style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.Header.Get("Origin") != "http://"+r.Host ||
				!strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				http.Error(w, "请求来源校验失败", http.StatusForbidden)
				return
			}
		}
		protected.ServeHTTP(w, r)
	})
}

func (h *consoleHandler) loginPage(w http.ResponseWriter, _ *http.Request) {
	page, err := fs.ReadFile(webui.Root(), "index.html")
	if err != nil {
		http.Error(w, "登录页面不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(page)
}

func respond(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(data)
}

func (h *consoleHandler) changePassword(w http.ResponseWriter, r *http.Request) {
	if !h.admin.allowAttempt(r) {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "尝试过于频繁，请一分钟后再试", http.StatusTooManyRequests)
		return
	}
	var body struct {
		Current string `json:"current"`
		Next    string `json:"next"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if err := h.admin.ChangePassword(body.Current, body.Next); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.manager.Record("管理员", "管理密码已修改，所有控制台会话已失效")
	respond(w, map[string]bool{"ok": true})
}

func (h *consoleHandler) status(w http.ResponseWriter, _ *http.Request) {
	respond(w, h.manager.Snapshot())
}

func (h *consoleHandler) resourcePacks(w http.ResponseWriter, r *http.Request) {
	list, err := h.manager.ResourcePacks(r.URL.Query().Get("profile_id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, list)
}

func (h *consoleHandler) refreshResourcePacks(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProfileID string `json:"profile_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	result, err := h.manager.RefreshResourcePacks(ctx, body.ProfileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, result)
}

func (h *consoleHandler) start(w http.ResponseWriter, _ *http.Request) {
	if err := h.manager.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, map[string]bool{"ok": true})
}

func (h *consoleHandler) stop(w http.ResponseWriter, _ *http.Request) {
	h.manager.Stop()
	respond(w, map[string]bool{"ok": true})
}

func (h *consoleHandler) startRoom(w http.ResponseWriter, r *http.Request) {
	h.roomAction(w, r, h.manager.StartAccount)
}

func (h *consoleHandler) stopRoom(w http.ResponseWriter, r *http.Request) {
	h.roomAction(w, r, h.manager.StopAccount)
}

func (h *consoleHandler) restartRoom(w http.ResponseWriter, r *http.Request) {
	h.roomAction(w, r, h.manager.RestartAccount)
}

func (h *consoleHandler) roomAction(w http.ResponseWriter, r *http.Request, action func(int) error) {
	var body struct {
		Index int `json:"index"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if err := action(body.Index); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, map[string]bool{"ok": true})
}

func (h *consoleHandler) assignProfile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Index     int    `json:"index"`
		ProfileID string `json:"profile_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if err := h.manager.AssignAccountProfile(body.Index, body.ProfileID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.manager.Record("账号", fmt.Sprintf("房间 %d 已切换配置方案：%s", body.Index+1, body.ProfileID))
	respond(w, map[string]any{"ok": true, "index": body.Index, "profile_id": body.ProfileID})
}

func (h *consoleHandler) setDebug(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || body.Enabled == nil {
		http.Error(w, "请指定 enabled 为 true 或 false", http.StatusBadRequest)
		return
	}
	if err := h.manager.SetDebug(*body.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respond(w, map[string]bool{"ok": true})
}

func (h *consoleHandler) runConsole(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command string `json:"command"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	result, err := h.manager.RunCommand(body.Command)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, result)
}

func (h *consoleHandler) exit(w http.ResponseWriter, _ *http.Request) {
	respond(w, map[string]bool{"ok": true})
	h.manager.RequestExit()
}

func (h *consoleHandler) saveConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Config config.Config `json:"config"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		http.Error(w, "配置 JSON 无效", http.StatusBadRequest)
		return
	}
	if err := h.manager.Configure(body.Config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, map[string]bool{"ok": true})
}

// saveSession stores one verified session and its account metadata. index
// selects an existing room to replace; nil adds a new account.
func (h *consoleHandler) saveSession(token, platform, expiresAt, email, label, profileID string, profile auth.Profile, index *int) (int, error) {
	meta := core.AccountMeta{
		Email: strings.TrimSpace(email), Label: strings.TrimSpace(label),
		Name: profile.Name, UID: profile.UID, Level: profile.Level,
		Platform: platform, ExpiresAt: expiresAt, ProfileID: strings.TrimSpace(profileID),
	}
	if index != nil {
		existing := h.manager.Accounts()
		if *index < 0 || *index >= len(existing) {
			return -1, fmt.Errorf("账号序号无效")
		}
		if meta.Email == "" {
			meta.Email = existing[*index].Email
		}
		if meta.Label == "" {
			meta.Label = existing[*index].Label
		}
	}
	if meta.Email == "" {
		meta.Email = meta.Label
	}
	if meta.Email == "" {
		meta.Email = profile.Name
	}
	if meta.Label == "" {
		meta.Label = "互通版会话"
	}
	if meta.Email == "" {
		meta.Email = "互通版会话"
	}

	if index != nil {
		if err := h.manager.ReplaceAccount(*index, token, meta); err != nil {
			return -1, err
		}
		h.manager.Record("账号", fmt.Sprintf("已更新房间 %d 的互通版会话：%s", *index+1, profile.Name))
		return *index, nil
	}
	created, err := h.manager.AddAccount(token, meta)
	if err != nil {
		return -1, err
	}
	h.manager.Record("账号", fmt.Sprintf("已导入互通版会话：%s（UID %s）→ 房间 %d", profile.Name, profile.UID, created+1))
	return created, nil
}

func (h *consoleHandler) importSession(w http.ResponseWriter, r *http.Request) {
	if !loopbackRequest(r) {
		http.Error(w, "为避免会话经网络明文传输，网易账号登录仅允许本机访问控制台", http.StatusForbidden)
		return
	}
	if h.manager.Snapshot().Running {
		// Authenticate only after the room is stopped: a second NetEase login
		// can invalidate the session that is keeping the running room alive.
		http.Error(w, "请先停止房间，再添加或更新账号", http.StatusBadRequest)
		return
	}
	var body struct {
		Sauth     string `json:"sauth"`
		Email     string `json:"email"`
		Label     string `json:"label"`
		Index     *int   `json:"index"`
		ProfileID string `json:"profile_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	token, platform, expiresAt, err := core.NormalizeInteropSession(body.Sauth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	profile, err := auth.LookupProfile(ctx, token)
	if err != nil {
		message := redact.Sanitize(err.Error(), token)
		h.manager.Record("账号", "互通版会话验证失败："+message)
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	index, err := h.saveSession(token, platform, expiresAt, body.Email, body.Label, body.ProfileID, profile, body.Index)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result := map[string]any{
		"ok": true, "index": index, "name": profile.Name, "uid": profile.UID,
		"level": profile.Level, "platform": platform, "expires_at": expiresAt,
		"profile_id": h.accountProfileID(index),
	}
	if expiresAt == "" {
		delete(result, "expires_at")
	}
	respond(w, result)
}

func (h *consoleHandler) interopLogin(w http.ResponseWriter, r *http.Request) {
	if !loopbackRequest(r) {
		http.Error(w, "为避免密码经网络明文传输，网易账号登录仅允许本机访问控制台", http.StatusForbidden)
		return
	}
	if h.manager.Snapshot().Running {
		http.Error(w, "请先停止房间，再添加或更新账号", http.StatusBadRequest)
		return
	}
	var body struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		Label     string `json:"label"`
		Index     *int   `json:"index"`
		ProfileID string `json:"profile_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(body.Email)
	if email == "" || body.Password == "" {
		http.Error(w, "请填写网易邮箱和密码", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	sauth, err := h.interop.Login(ctx, email, body.Password)
	if err != nil {
		var verify *interoplogin.VerifyRequired
		if errors.As(err, &verify) {
			h.manager.Record("账号", "互通版直登需要安全验证："+verify.Message)
			respond(w, map[string]any{
				"status": "need_verify", "message": verify.Message, "verify_url": verify.URL,
			})
			return
		}
		message := redact.Sanitize(err.Error(), body.Password)
		h.manager.Record("账号", "互通版直登失败："+message)
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	inner, err := json.Marshal(sauth)
	if err != nil {
		http.Error(w, "序列化会话失败", http.StatusInternalServerError)
		return
	}
	token, platform, expiresAt, err := core.NormalizeInteropSession(string(inner))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	profile, err := auth.LookupProfile(ctx, token)
	if err != nil {
		message := redact.Sanitize(err.Error(), token)
		h.manager.Record("账号", "互通版会话验证失败："+message)
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	index, err := h.saveSession(token, platform, expiresAt, email, body.Label, body.ProfileID, profile, body.Index)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result := map[string]any{
		"ok": true, "index": index, "name": profile.Name, "uid": profile.UID,
		"level": profile.Level, "platform": platform, "expires_at": expiresAt,
		"profile_id": h.accountProfileID(index),
	}
	if expiresAt == "" {
		delete(result, "expires_at")
	}
	respond(w, result)
}

func (h *consoleHandler) removeAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Index int `json:"index"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if err := h.manager.RemoveAccount(body.Index); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.manager.Record("账号", fmt.Sprintf("已移除房间 %d 的网易账号", body.Index+1))
	respond(w, map[string]bool{"ok": true})
}

func (h *consoleHandler) refreshProfile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Index int `json:"index"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if h.manager.Snapshot().Running {
		http.Error(w, "请先停止房间，再验证账号", http.StatusBadRequest)
		return
	}
	token, err := h.manager.Token(body.Index)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	profile, fetchErr := auth.LookupProfile(ctx, token)
	if fetchErr != nil {
		message := redact.Sanitize(fetchErr.Error(), token)
		_ = h.manager.UpdateAccountProfile(body.Index, "", "", 0, message)
		h.manager.Record("账号", fmt.Sprintf("房间 %d 游戏名解析失败：%s", body.Index+1, message))
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	if err := h.manager.UpdateAccountProfile(body.Index, profile.Name, profile.UID, profile.Level, ""); err != nil {
		http.Error(w, "保存账号资料失败", http.StatusInternalServerError)
		return
	}
	h.manager.Record("账号", fmt.Sprintf("房间 %d 游戏名解析成功：%s（UID %s）", body.Index+1, profile.Name, profile.UID))
	respond(w, map[string]any{"ok": true, "name": profile.Name, "uid": profile.UID, "level": profile.Level})
}

func (h *consoleHandler) accountProfileID(index int) string {
	accounts := h.manager.Accounts()
	if index < 0 || index >= len(accounts) {
		return ""
	}
	return accounts[index].ProfileID
}

func loopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
