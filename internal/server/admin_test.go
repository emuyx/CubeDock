package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/core"
)

func newConfiguredAdmin(t *testing.T, username, password string) *Admin {
	t.Helper()
	admin, err := LoadAdmin(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !admin.NeedsSetup() {
		t.Fatal("fresh admin should require setup")
	}
	if err := admin.CompleteSetup(username, password); err != nil {
		t.Fatal(err)
	}
	return admin
}

func TestAdminLoginProtectAndPasswordChange(t *testing.T) {
	admin := newConfiguredAdmin(t, "owner", "correct-horse")

	login := func(username, password string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"username": username, "password": password})
		request := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(string(body)))
		request.RemoteAddr = "127.0.0.1:4000"
		record := httptest.NewRecorder()
		admin.login(record, request)
		return record
	}
	if wrong := login("owner", "wrong-password"); wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d", wrong.Code)
	}
	if wrong := login("admin", "correct-horse"); wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong username = %d", wrong.Code)
	}
	record := login("owner", "correct-horse")
	if record.Code != http.StatusOK {
		t.Fatalf("login = %d: %s", record.Code, record.Body.String())
	}
	cookies := record.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != adminCookieName || !cookies[0].HttpOnly {
		t.Fatalf("session cookie = %+v", cookies)
	}

	protected := admin.Protect(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	request.AddCookie(cookies[0])
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("authenticated request = %d %q", response.Code, response.Body.String())
	}

	if err := admin.ChangePassword("wrong", "new-password"); err == nil {
		t.Fatal("wrong current password accepted")
	}
	if err := admin.ChangePassword("correct-horse", "new-password"); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("old session survived password change: %d", response.Code)
	}
	if record := login("owner", "new-password"); record.Code != http.StatusOK {
		t.Fatalf("new password login = %d", record.Code)
	}
}

func TestAdminSetupFlow(t *testing.T) {
	manager, err := core.New(t.TempDir(), config.Defaults(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	adminDir := t.TempDir()
	admin, err := LoadAdmin(adminDir)
	if err != nil {
		t.Fatal(err)
	}
	handler := &consoleHandler{manager: manager, admin: admin}

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	statusRequest.RemoteAddr = "127.0.0.1:4000"
	status := httptest.NewRecorder()
	handler.setupStatus(status, statusRequest)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"required":true`) {
		t.Fatalf("setup status = %d %s", status.Code, status.Body.String())
	}

	loginBefore := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"owner","password":"correct-horse"}`))
	loginBefore.RemoteAddr = "127.0.0.1:4000"
	loginRecord := httptest.NewRecorder()
	admin.login(loginRecord, loginBefore)
	if loginRecord.Code != http.StatusConflict {
		t.Fatalf("login before setup = %d", loginRecord.Code)
	}

	remote := httptest.NewRequest(http.MethodPost, "/api/setup",
		strings.NewReader(`{"username":"owner","password":"correct-horse","confirm":"correct-horse"}`))
	remote.RemoteAddr = "192.0.2.10:4000"
	remoteRecord := httptest.NewRecorder()
	handler.completeSetup(remoteRecord, remote)
	if remoteRecord.Code != http.StatusForbidden {
		t.Fatalf("remote setup = %d", remoteRecord.Code)
	}
	if !admin.NeedsSetup() {
		t.Fatal("remote setup created the account")
	}

	mismatch := httptest.NewRequest(http.MethodPost, "/api/setup",
		strings.NewReader(`{"username":"owner","password":"correct-horse","confirm":"different"}`))
	mismatch.RemoteAddr = "127.0.0.1:4000"
	mismatchRecord := httptest.NewRecorder()
	handler.completeSetup(mismatchRecord, mismatch)
	if mismatchRecord.Code != http.StatusBadRequest {
		t.Fatalf("mismatched setup = %d", mismatchRecord.Code)
	}

	setup := httptest.NewRequest(http.MethodPost, "/api/setup",
		strings.NewReader(`{"username":"owner","password":"correct-horse","confirm":"correct-horse"}`))
	setup.RemoteAddr = "127.0.0.1:4000"
	setupRecord := httptest.NewRecorder()
	handler.completeSetup(setupRecord, setup)
	if setupRecord.Code != http.StatusOK {
		t.Fatalf("setup = %d: %s", setupRecord.Code, setupRecord.Body.String())
	}
	if admin.NeedsSetup() || admin.Username() != "owner" {
		t.Fatalf("admin after setup = %q setup=%v", admin.Username(), admin.NeedsSetup())
	}
	if len(setupRecord.Result().Cookies()) == 0 {
		t.Fatal("setup did not issue a session")
	}

	again := httptest.NewRequest(http.MethodPost, "/api/setup",
		strings.NewReader(`{"username":"other","password":"another-password","confirm":"another-password"}`))
	again.RemoteAddr = "127.0.0.1:4000"
	againRecord := httptest.NewRecorder()
	handler.completeSetup(againRecord, again)
	if againRecord.Code != http.StatusConflict {
		t.Fatalf("second setup = %d", againRecord.Code)
	}

	reloaded, err := LoadAdmin(adminDir)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.NeedsSetup() || reloaded.Username() != "owner" {
		t.Fatalf("reloaded admin = %q setup=%v", reloaded.Username(), reloaded.NeedsSetup())
	}
	if !strings.Contains(setupRecord.Body.String(), `"username":"owner"`) {
		t.Fatalf("setup response = %s", setupRecord.Body.String())
	}
}

func TestLoopbackRequest(t *testing.T) {
	for address, want := range map[string]bool{
		"127.0.0.1:5000": true,
		"[::1]:5000":     true,
		"192.0.2.10:80":  false,
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/session/import", nil)
		request.RemoteAddr = address
		if got := loopbackRequest(request); got != want {
			t.Fatalf("%s = %v, want %v", address, got, want)
		}
	}
}
