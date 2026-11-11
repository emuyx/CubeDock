package server

import (
	"encoding/json"
	"net/http"
)

// setupStatus is public so the login page can decide between first-run setup
// and the normal login form. It never exposes hashes or sessions.
func (h *consoleHandler) setupStatus(w http.ResponseWriter, r *http.Request) {
	// Only reveal the default name during first-run setup. After setup the
	// configured username is not public.
	username := ""
	if h.admin.NeedsSetup() {
		username = h.admin.Username()
	}
	respond(w, map[string]any{
		"required": h.admin.NeedsSetup(),
		"local":    loopbackRequest(r),
		"username": username,
	})
}

// completeSetup creates the first administrator account. It is loopback-only:
// the server may listen on a LAN address, but a remote visitor must not be able
// to claim the console during first run.
func (h *consoleHandler) completeSetup(w http.ResponseWriter, r *http.Request) {
	if !loopbackRequest(r) {
		http.Error(w, "请在本机控制台完成管理员初始化", http.StatusForbidden)
		return
	}
	if !h.admin.NeedsSetup() {
		http.Error(w, "管理员账户已经初始化", http.StatusConflict)
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Confirm  string `json:"confirm"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		http.Error(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if body.Password != body.Confirm {
		http.Error(w, "两次输入的密码不一致", http.StatusBadRequest)
		return
	}
	if err := h.admin.CompleteSetup(body.Username, body.Password); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, _, version := h.admin.credentials()
	if err := h.admin.issueSession(w, r, version); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.manager.Record("管理员", "管理员账户已创建："+h.admin.Username())
	respond(w, map[string]any{"ok": true, "username": h.admin.Username()})
}
