// Package server adapts the CubeDock core to the browser console.
package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/console"
	"github.com/emuyx/cubedock/internal/core"
	"github.com/emuyx/cubedock/internal/interoplogin"
	"github.com/emuyx/cubedock/internal/m4399login"
	"github.com/emuyx/cubedock/internal/netease"
	"github.com/emuyx/cubedock/internal/redact"
	"github.com/emuyx/cubedock/internal/webui"
)

type mobileLoginService interface {
	Login(ctx context.Context, email, password string) (map[string]any, error)
}

type m4399LoginService interface {
	Login(ctx context.Context, username, password, captchaID, captcha string) (map[string]any, error)
}

type consoleHandler struct {
	manager *core.Manager
	admin   *Admin
	interop *interoplogin.Service
	mobile  mobileLoginService
	m4399   m4399LoginService
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
		mobile:  netease.NewMobileLoginService(manager.Dir()),
		m4399:   m4399login.New(),
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
	mux.HandleFunc("POST /api/session/mobile-login", h.mobileLogin)
	mux.HandleFunc("POST /api/session/4399-login", h.login4399)
	mux.HandleFunc("POST /api/account/remove", h.removeAccount)
	mux.HandleFunc("POST /api/account/profile", h.refreshProfile)
	mux.HandleFunc("POST /api/account/set-name", h.setAccountName)
	mux.HandleFunc("POST /api/account/assign-profile", h.assignProfile)
	mux.HandleFunc("GET /api/account/password", h.accountPassword)
	mux.HandleFunc("POST /api/account/password", h.saveAccountPassword)
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
				"style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; "+
				"img-src 'self' data: https:; frame-ancestors 'none'")
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

// errAccountChanged protects index-based account actions when another UI
// removed or reordered accounts between the snapshot and the click.
var errAccountChanged = errors.New("账号列表已变化，请刷新页面后重试")

func (h *consoleHandler) validateAccount(index int, uid, email string) error {
	if uid == "" && email == "" {
		return nil
	}
	accounts := h.manager.Accounts()
	if index < 0 || index >= len(accounts) {
		return errAccountChanged
	}
	account := accounts[index]
	// UID is stable across re-login; email is the fallback for accounts whose
	// profile has not been resolved yet.
	if uid != "" {
		if account.UID != uid {
			return errAccountChanged
		}
		return nil
	}
	if account.Email != email {
		return errAccountChanged
	}
	return nil
}

func (h *consoleHandler) accountError(w http.ResponseWriter, err error) {
	if errors.Is(err, errAccountChanged) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	http.Error(w, err.Error(), http.StatusBadRequest)
}

func (h *consoleHandler) roomAction(w http.ResponseWriter, r *http.Request, action func(int) error) {
	var body struct {
		Index int    `json:"index"`
		UID   string `json:"uid"`
		Email string `json:"email"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if err := h.validateAccount(body.Index, body.UID, body.Email); err != nil {
		h.accountError(w, err)
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
		UID       string `json:"uid"`
		Email     string `json:"email"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if err := h.validateAccount(body.Index, body.UID, body.Email); err != nil {
		h.accountError(w, err)
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
		Config   config.Config `json:"config"`
		Revision *uint64       `json:"revision"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		http.Error(w, "配置 JSON 无效", http.StatusBadRequest)
		return
	}
	var err error
	if body.Revision != nil {
		err = h.manager.ConfigureIfRevision(body.Config, *body.Revision)
	} else {
		err = h.manager.Configure(body.Config)
	}
	if errors.Is(err, core.ErrConfigChanged) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, map[string]any{"ok": true, "config_revision": h.manager.Snapshot().ConfigRevision})
}

// saveSession stores one verified session and its account metadata. index
// selects an existing room to replace; nil adds a new account. profileErr is
// set when the account exists but has no game name yet.
func (h *consoleHandler) saveSession(token, platform, authMethod, expiresAt, email, label, profileID, profileErr string, profile auth.Profile, index *int) (int, error) {
	meta := core.AccountMeta{
		Email: strings.TrimSpace(email), Label: strings.TrimSpace(label),
		Name: profile.Name, UID: profile.UID, Level: profile.Level, AvatarURL: profile.AvatarURL,
		Platform: platform, AuthMethod: authMethod, ExpiresAt: expiresAt,
		ProfileError: profileErr,
		ProfileID:    strings.TrimSpace(profileID),
	}
	methodLabel := netease.AuthMethodLabel(netease.AuthMethod(authMethod))
	displayName := profile.Name
	if displayName == "" {
		displayName = "未命名账号"
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
		meta.Label = methodLabel + "会话"
	}
	if meta.Email == "" {
		meta.Email = methodLabel + "会话"
	}

	if index != nil {
		if err := h.manager.ReplaceAccount(*index, token, meta); err != nil {
			return -1, err
		}
		if profileErr != "" {
			h.manager.Record("账号", fmt.Sprintf("已更新房间 %d 的%s会话，但账号还没有游戏名，需要先设置游戏名", *index+1, methodLabel))
		} else {
			h.manager.Record("账号", fmt.Sprintf("已更新房间 %d 的%s会话：%s", *index+1, methodLabel, displayName))
		}
		return *index, nil
	}
	created, err := h.manager.AddAccount(token, meta)
	if err != nil {
		return -1, err
	}
	if profileErr != "" {
		h.manager.Record("账号", fmt.Sprintf("已导入%s会话：%s（UID %s）→ 房间 %d；需要先设置游戏名",
			methodLabel, displayName, profile.UID, created+1))
	} else {
		h.manager.Record("账号", fmt.Sprintf("已导入%s会话：%s（UID %s）→ 房间 %d", methodLabel, displayName, profile.UID, created+1))
	}
	return created, nil
}

// lookupSessionProfile resolves the account identity. A new account without a
// game name is not a fatal error: it is returned with a profileErr so the
// session can be saved and the naming flow offered.
func (h *consoleHandler) lookupSessionProfile(ctx context.Context, method netease.AuthMethod, token, email string) (auth.Profile, string, string) {
	profile, err := auth.LookupProfileFor(ctx, method, token)
	if err == nil {
		return profile, "", ""
	}
	if errors.Is(err, netease.ErrProfileNameMissing) {
		return profile, netease.ErrProfileNameMissing.Error(), ""
	}
	message := redact.Sanitize(err.Error(), token)
	if email != "" {
		message = redact.Sanitize(message, email)
	}
	h.manager.Record("账号", netease.AuthMethodLabel(method)+"会话验证失败："+message)
	return profile, "", message
}

// respondSession saves a verified session (possibly without a game name) and
// returns the console response shape shared by all login methods.
func (h *consoleHandler) respondSession(w http.ResponseWriter, token, platform, authMethod, expiresAt, email, label, profileID string, profile auth.Profile, profileErr string, index *int) {
	savedIndex, err := h.saveSession(token, platform, authMethod, expiresAt, email, label, profileID, profileErr, profile, index)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result := map[string]any{
		"ok": true, "index": savedIndex, "name": profile.Name, "uid": profile.UID,
		"level": profile.Level, "avatar_url": profile.AvatarURL,
		"platform": platform, "auth_method": authMethod,
		"expires_at": expiresAt, "profile_id": h.accountProfileID(savedIndex),
	}
	if expiresAt == "" {
		delete(result, "expires_at")
	}
	if profileErr != "" {
		result["status"] = "need_name"
		result["message"] = "该网易账号还没有设置游戏名，请先设置游戏名"
		result["profile_error"] = profileErr
	}
	respond(w, result)
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
		Sauth      string `json:"sauth"`
		AuthMethod string `json:"auth_method"`
		Email      string `json:"email"`
		Label      string `json:"label"`
		Index      *int   `json:"index"`
		ProfileID  string `json:"profile_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	token, method, expiresAt, err := core.NormalizeSession(body.Sauth, body.AuthMethod)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	authMethod := netease.AuthMethod(method)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	profile, profileErr, lookupErr := h.lookupSessionProfile(ctx, authMethod, token, body.Email)
	if lookupErr != "" {
		http.Error(w, lookupErr, http.StatusBadRequest)
		return
	}
	h.respondSession(w, token, method, method, expiresAt, body.Email, body.Label, body.ProfileID, profile, profileErr, body.Index)
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
	token, method, expiresAt, err := core.NormalizeSession(string(inner), string(netease.AuthInterop))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	profile, profileErr, lookupErr := h.lookupSessionProfile(ctx, netease.AuthInterop, token, email)
	if lookupErr != "" {
		http.Error(w, lookupErr, http.StatusBadRequest)
		return
	}
	h.respondSession(w, token, method, method, expiresAt, email, body.Label, body.ProfileID, profile, profileErr, body.Index)
}

func (h *consoleHandler) mobileLogin(w http.ResponseWriter, r *http.Request) {
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
	if h.mobile == nil {
		http.Error(w, "手游登录服务不可用", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	sauth, err := h.mobile.Login(ctx, email, body.Password)
	if err != nil {
		var verify *netease.MobileVerifyRequired
		if errors.As(err, &verify) {
			h.manager.Record("账号", "手游直登需要安全验证："+verify.Message)
			respond(w, map[string]any{
				"status": "need_verify", "message": verify.Message, "verify_url": verify.URL,
			})
			return
		}
		message := redact.Sanitize(err.Error(), body.Password)
		message = redact.Sanitize(message, email)
		h.manager.Record("账号", "手游直登失败："+message)
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	inner, err := json.Marshal(sauth)
	if err != nil {
		http.Error(w, "序列化会话失败", http.StatusInternalServerError)
		return
	}
	token, method, expiresAt, err := core.NormalizeSession(string(inner), string(netease.AuthMobileNetEase))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	profile, profileErr, lookupErr := h.lookupSessionProfile(ctx, netease.AuthMobileNetEase, token, email)
	if lookupErr != "" {
		http.Error(w, lookupErr, http.StatusBadRequest)
		return
	}
	h.respondSession(w, token, method, method, expiresAt, email, body.Label, body.ProfileID, profile, profileErr, body.Index)
}

func (h *consoleHandler) login4399(w http.ResponseWriter, r *http.Request) {
	if !loopbackRequest(r) {
		http.Error(w, "为避免密码经网络明文传输，网易账号登录仅允许本机访问控制台", http.StatusForbidden)
		return
	}
	if h.manager.Snapshot().Running {
		http.Error(w, "请先停止房间，再添加或更新账号", http.StatusBadRequest)
		return
	}
	var body struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		CaptchaID string `json:"captcha_id"`
		Captcha   string `json:"captcha"`
		Label     string `json:"label"`
		Index     *int   `json:"index"`
		ProfileID string `json:"profile_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	username := strings.TrimSpace(body.Username)
	if username == "" || body.Password == "" {
		http.Error(w, "请填写 4399 账号和密码", http.StatusBadRequest)
		return
	}
	if h.m4399 == nil {
		http.Error(w, "4399 登录服务不可用", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	sauth, err := h.m4399.Login(ctx, username, body.Password,
		strings.TrimSpace(body.CaptchaID), strings.TrimSpace(body.Captcha))
	if err != nil {
		var captcha *m4399login.CaptchaRequired
		if errors.As(err, &captcha) {
			message := captcha.Message
			if message == "" {
				message = "请输入图形验证码"
			}
			imageType := captcha.ImageType
			if imageType == "" {
				imageType = "image/png"
			}
			h.manager.Record("账号", "4399 登录需要图形验证码")
			respond(w, map[string]any{
				"status":             "need_captcha",
				"captcha_id":         captcha.CaptchaID,
				"captcha_image":      base64.StdEncoding.EncodeToString(captcha.Image),
				"captcha_image_type": imageType,
				"message":            message,
			})
			return
		}
		message := redact.Sanitize(err.Error(), body.Password)
		message = redact.Sanitize(message, username)
		h.manager.Record("账号", "4399 直登失败："+message)
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	inner, err := json.Marshal(sauth)
	if err != nil {
		http.Error(w, "序列化会话失败", http.StatusInternalServerError)
		return
	}
	token, method, expiresAt, err := core.NormalizeSession(string(inner), string(netease.AuthMobile4399))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	verifyCtx, verifyCancel := context.WithTimeout(r.Context(), 75*time.Second)
	defer verifyCancel()
	profile, profileErr, lookupErr := h.lookupSessionProfile(verifyCtx, netease.AuthMobile4399, token, username)
	if lookupErr != "" {
		http.Error(w, lookupErr, http.StatusBadRequest)
		return
	}
	h.respondSession(w, token, method, method, expiresAt, username, body.Label, body.ProfileID, profile, profileErr, body.Index)
}

func (h *consoleHandler) removeAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Index int    `json:"index"`
		UID   string `json:"uid"`
		Email string `json:"email"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if err := h.validateAccount(body.Index, body.UID, body.Email); err != nil {
		h.accountError(w, err)
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
		Index int    `json:"index"`
		UID   string `json:"uid"`
		Email string `json:"email"`
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
	if err := h.validateAccount(body.Index, body.UID, body.Email); err != nil {
		h.accountError(w, err)
		return
	}
	token, err := h.manager.Token(body.Index)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	accounts := h.manager.Accounts()
	if body.Index < 0 || body.Index >= len(accounts) {
		http.Error(w, "账号序号无效", http.StatusBadRequest)
		return
	}
	method, err := netease.ParseAuthMethod(accounts[body.Index].AuthMethod)
	if err != nil {
		http.Error(w, "账号登录方式无效", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	profile, fetchErr := auth.LookupProfileFor(ctx, method, token)
	if fetchErr != nil {
		message := redact.Sanitize(fetchErr.Error(), token)
		_ = h.manager.UpdateAccountProfile(body.Index, "", "", 0, "", message)
		h.manager.Record("账号", fmt.Sprintf("房间 %d 游戏名解析失败：%s", body.Index+1, message))
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	if err := h.manager.UpdateAccountProfile(body.Index, profile.Name, profile.UID, profile.Level, profile.AvatarURL, ""); err != nil {
		http.Error(w, "保存账号资料失败", http.StatusInternalServerError)
		return
	}
	h.manager.Record("账号", fmt.Sprintf("房间 %d 游戏名解析成功：%s（UID %s）", body.Index+1, profile.Name, profile.UID))
	respond(w, map[string]any{"ok": true, "name": profile.Name, "uid": profile.UID, "level": profile.Level, "avatar_url": profile.AvatarURL})
}

// setAccountName sets the game name for a new account that has no name yet.
// The session was already saved by the login flow; this endpoint only needs
// the stored token and the operator-supplied name.
func (h *consoleHandler) setAccountName(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Index int    `json:"index"`
		Name  string `json:"name"`
		UID   string `json:"uid"`
		Email string `json:"email"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if h.manager.Snapshot().Running {
		http.Error(w, "请先停止房间，再设置游戏名", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(body.Name)
	if body.Index < 0 || name == "" {
		http.Error(w, "请指定账号序号和游戏名", http.StatusBadRequest)
		return
	}
	if err := h.validateAccount(body.Index, body.UID, body.Email); err != nil {
		h.accountError(w, err)
		return
	}
	accounts := h.manager.Accounts()
	if body.Index >= len(accounts) {
		http.Error(w, "账号序号无效", http.StatusBadRequest)
		return
	}
	token, err := h.manager.Token(body.Index)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	method, err := netease.ParseAuthMethod(accounts[body.Index].AuthMethod)
	if err != nil {
		http.Error(w, "账号登录方式无效", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 75*time.Second)
	defer cancel()
	profile, err := auth.SetPlayerName(ctx, method, token, name)
	if err != nil {
		message := redact.Sanitize(err.Error(), token)
		h.manager.Record("账号", fmt.Sprintf("房间 %d 设置游戏名失败：%s", body.Index+1, message))
		http.Error(w, message, http.StatusBadRequest)
		return
	}
	if err := h.manager.UpdateAccountProfile(body.Index, profile.Name, profile.UID, profile.Level, profile.AvatarURL, ""); err != nil {
		http.Error(w, "保存账号资料失败", http.StatusInternalServerError)
		return
	}
	h.manager.Record("账号", fmt.Sprintf("房间 %d 已设置游戏名：%s", body.Index+1, profile.Name))
	respond(w, map[string]any{
		"ok": true, "index": body.Index, "name": profile.Name,
		"uid": profile.UID, "level": profile.Level, "avatar_url": profile.AvatarURL,
	})
}

func (h *consoleHandler) accountPassword(w http.ResponseWriter, r *http.Request) {
	if !loopbackRequest(r) {
		http.Error(w, "本地保存的密码仅允许本机控制台读取", http.StatusForbidden)
		return
	}
	index, err := strconv.Atoi(r.URL.Query().Get("index"))
	if err != nil || index < 0 {
		http.Error(w, "账号序号无效", http.StatusBadRequest)
		return
	}
	password, saved, err := h.manager.AccountPassword(index)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, map[string]any{"saved": saved, "password": password})
}

func (h *consoleHandler) saveAccountPassword(w http.ResponseWriter, r *http.Request) {
	if !loopbackRequest(r) {
		http.Error(w, "本地保存的密码仅允许本机控制台写入", http.StatusForbidden)
		return
	}
	var body struct {
		Index    int    `json:"index"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || body.Index < 0 {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if err := h.manager.SaveAccountPassword(body.Index, body.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	respond(w, map[string]any{"ok": true, "saved": body.Password != ""})
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
