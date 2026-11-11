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

func newDiagnosticsTestManager(t *testing.T) *core.Manager {
	t.Helper()
	manager, err := core.New(t.TempDir(), config.Defaults(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Upstream = "127.0.0.1:1"
	cfg.LimboPacksDir = t.TempDir()
	if err := manager.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	return manager
}

func TestDiagnosticsEndpoint(t *testing.T) {
	manager := newDiagnosticsTestManager(t)
	handler := &consoleHandler{manager: manager}
	recorder := httptest.NewRecorder()
	handler.diagnostics(recorder, httptest.NewRequest(http.MethodGet, "/api/diagnostics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		GeneratedAt string `json:"generated_at"`
		Runtime     struct {
			Version string `json:"version"`
			DataDir string `json:"data_dir"`
		} `json:"runtime"`
		Summary map[string]int `json:"summary"`
		Checks  []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.GeneratedAt == "" || response.Runtime.Version == "" || response.Runtime.DataDir != manager.Dir() {
		t.Fatalf("runtime = %+v", response.Runtime)
	}
	if len(response.Checks) == 0 || len(response.Summary) == 0 {
		t.Fatalf("checks = %+v summary = %+v", response.Checks, response.Summary)
	}
	seen := map[string]bool{}
	for _, check := range response.Checks {
		seen[check.ID] = true
		if check.Status == "" {
			t.Fatalf("check without status: %+v", check)
		}
	}
	for _, id := range []string{"config", "upstream", "packs", "accounts", "locks", "webui", "logs"} {
		if !seen[id] {
			t.Fatalf("missing check %q", id)
		}
	}
	if strings.Contains(recorder.Body.String(), "sauth") || strings.Contains(recorder.Body.String(), "token-") {
		t.Fatal("diagnostics response contains credential material")
	}
}

func TestBackupEndpointExcludesSecrets(t *testing.T) {
	manager := newDiagnosticsTestManager(t)
	cfg := config.Defaults()
	cfg.Upstream = "127.0.0.1:1"
	cfg.LimboPacksDir = manager.Dir() + "/packs"
	cfg.RoomPassword = "room-secret"
	if err := manager.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.AddAccount("token-secret-1", core.AccountMeta{
		Email: "user@example.com", Name: "User", Platform: "interop",
	}); err != nil {
		t.Fatal(err)
	}
	handler := &consoleHandler{manager: manager, runtime: RuntimeInfo{Version: "test"}}

	recorder := httptest.NewRecorder()
	handler.backup(recorder, httptest.NewRequest(http.MethodGet, "/api/backup", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if disposition := recorder.Header().Get("Content-Disposition"); !strings.Contains(disposition, "attachment") {
		t.Fatalf("Content-Disposition = %q", disposition)
	}
	var payload backupFile
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Format != 1 || payload.Version != "test" || len(payload.Accounts) != 1 {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Config.RoomPassword != "" {
		t.Fatal("room password exported by default")
	}
	if strings.Contains(recorder.Body.String(), "token-secret-1") {
		t.Fatal("backup contains the account token")
	}

	recorder = httptest.NewRecorder()
	handler.backup(recorder, httptest.NewRequest(http.MethodGet, "/api/backup?include_password=true", nil))
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Config.RoomPassword != "room-secret" {
		t.Fatal("room password was not included when requested")
	}
}

func TestRestoreEndpointAppliesConfigAndKeepsPort(t *testing.T) {
	manager := newDiagnosticsTestManager(t)
	handler := &consoleHandler{manager: manager}
	currentPort := manager.Snapshot().Config.WebPort

	restored := config.Defaults()
	restored.Upstream = "127.0.0.1:19133"
	restored.WebPort = 19999
	restored.RoomName = "恢复后的房间"
	body, err := json.Marshal(map[string]any{"config": restored})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/restore", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.restore(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		OK       bool     `json:"ok"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || len(response.Warnings) == 0 {
		t.Fatalf("response = %+v", response)
	}
	snapshot := manager.Snapshot()
	if snapshot.Config.Upstream != restored.Upstream || snapshot.Config.RoomName != restored.RoomName {
		t.Fatalf("config after restore = %+v", snapshot.Config)
	}
	if snapshot.Config.WebPort != currentPort {
		t.Fatalf("web port changed to %d", snapshot.Config.WebPort)
	}
}

func TestRestoreRejectsInvalidConfig(t *testing.T) {
	manager := newDiagnosticsTestManager(t)
	handler := &consoleHandler{manager: manager}
	invalid := config.Defaults()
	invalid.ConnectionMode = "bogus"
	body, err := json.Marshal(map[string]any{"config": invalid})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/restore", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.restore(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestDiagnosticsRequiresAdminSession(t *testing.T) {
	manager := newDiagnosticsTestManager(t)
	admin, err := LoadAdmin(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.CompleteSetup("admin", "correct-horse"); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(manager, admin, RuntimeInfo{Version: "test"})
	request := httptest.NewRequest(http.MethodGet, "/api/diagnostics", nil)
	request.Host = "127.0.0.1:18321"
	request.RemoteAddr = "127.0.0.1:4000"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
