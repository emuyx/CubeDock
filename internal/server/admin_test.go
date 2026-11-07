package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminLoginProtectAndPasswordChange(t *testing.T) {
	admin, initial, err := LoadAdmin(t.TempDir(), "correct-horse", false)
	if err != nil {
		t.Fatal(err)
	}
	if initial != "" {
		t.Fatal("environment password was echoed")
	}

	login := func(password string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/login",
			strings.NewReader(`{"username":"admin","password":"`+password+`"}`))
		request.RemoteAddr = "127.0.0.1:4000"
		record := httptest.NewRecorder()
		admin.login(record, request)
		return record
	}
	wrong := login("wrong-password")
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d", wrong.Code)
	}
	record := login("correct-horse")
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
	if record := login("new-password"); record.Code != http.StatusOK {
		t.Fatalf("new password login = %d", record.Code)
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
