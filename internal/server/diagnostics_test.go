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
	cfg.Profiles[0].Upstream = "127.0.0.1:1"
	cfg.Profiles[0].LimboPacksDir = t.TempDir()
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
	cfg.Profiles[0].Upstream = "127.0.0.1:1"
	cfg.Profiles[0].LimboPacksDir = manager.Dir() + "/packs"
	cfg.Profiles[0].RoomPassword = "room-secret"
	second := config.DefaultProfile()
	second.ID = "second"
	second.Name = "第二方案"
	second.Upstream = "127.0.0.1:19134"
	second.RoomPassword = "second-secret"
	cfg.Profiles = append(cfg.Profiles, second)
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
	if payload.Format != 2 || payload.Version != "test" || len(payload.Accounts) != 1 {
		t.Fatalf("payload = %+v", payload)
	}
	for _, profile := range payload.Config.Profiles {
		if profile.RoomPassword != "" {
			t.Fatalf("room password exported by default: %+v", profile)
		}
	}
	if strings.Contains(recorder.Body.String(), "token-secret-1") {
		t.Fatal("backup contains the account token")
	}

	recorder = httptest.NewRecorder()
	handler.backup(recorder, httptest.NewRequest(http.MethodGet, "/api/backup?include_password=true", nil))
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Config.Profiles[0].RoomPassword != "room-secret" ||
		payload.Config.Profiles[1].RoomPassword != "second-secret" {
		t.Fatal("room passwords were not included when requested")
	}
}

func TestRestoreLegacyFormatMigrates(t *testing.T) {
	manager := newDiagnosticsTestManager(t)
	handler := &consoleHandler{manager: manager}
	legacy := map[string]any{
		"upstream": "127.0.0.1:19133", "room_name": "旧备份",
		"web_port": 18321, "retry_seconds": 10, "lobby_protocol_id": 43,
		"room_game_version": "1.21.130.0",
	}
	body, _ := json.Marshal(map[string]any{"config": legacy})
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
	if !response.OK || len(response.Warnings) == 0 || !strings.Contains(strings.Join(response.Warnings, " "), "旧版") {
		t.Fatalf("response = %+v", response)
	}
	snapshot := manager.Snapshot()
	if len(snapshot.Config.Profiles) != 1 ||
		snapshot.Config.Profiles[0].ID != config.DefaultProfileID ||
		snapshot.Config.Profiles[0].RoomName != "旧备份" {
		t.Fatalf("restored config = %+v", snapshot.Config)
	}
}

func TestAssignProfileEndpoint(t *testing.T) {
	manager := newDiagnosticsTestManager(t)
	cfg := manager.Snapshot().Config
	second := config.DefaultProfile()
	second.ID = "second"
	second.Name = "第二方案"
	second.Upstream = "127.0.0.1:19134"
	cfg.Profiles = append(cfg.Profiles, second)
	if err := manager.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.AddAccount("token-assign", core.AccountMeta{Email: "assign@example.com"}); err != nil {
		t.Fatal(err)
	}
	handler := &consoleHandler{manager: manager}
	body, _ := json.Marshal(map[string]any{"index": 0, "profile_id": "second"})
	request := httptest.NewRequest(http.MethodPost, "/api/account/assign-profile", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.assignProfile(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	accounts := manager.Accounts()
	if len(accounts) != 1 || accounts[0].ProfileID != "second" || accounts[0].ProfileName != "第二方案" {
		t.Fatalf("account after assign = %+v", accounts)
	}
}

func TestRestoreEndpointAppliesConfigAndKeepsPort(t *testing.T) {
	manager := newDiagnosticsTestManager(t)
	handler := &consoleHandler{manager: manager}
	currentPort := manager.Snapshot().Config.WebPort

	restored := config.Defaults()
	restored.Profiles[0].Upstream = "127.0.0.1:19133"
	restored.WebPort = 19999
	restored.Profiles[0].RoomName = "恢复后的房间"
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
	if snapshot.Config.Profiles[0].Upstream != restored.Profiles[0].Upstream ||
		snapshot.Config.Profiles[0].RoomName != restored.Profiles[0].RoomName {
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
	invalid.Profiles[0].ConnectionMode = "bogus"
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

func TestAdminDiagnosticHintMatchesRecoveryPath(t *testing.T) {
	manager := newDiagnosticsTestManager(t)
	admin, err := LoadAdmin(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := &consoleHandler{manager: manager, admin: admin}
	for _, check := range handler.consoleChecks() {
		if check.ID != "admin" {
			continue
		}
		if check.Status != core.DiagnosticFail {
			t.Fatalf("admin check status = %q, want fail", check.Status)
		}
		if strings.Contains(check.Hint, "reset-admin-password") {
			t.Fatalf("hint still references the removed CLI flag: %q", check.Hint)
		}
		if !strings.Contains(check.Hint, "admin.json") {
			t.Fatalf("hint does not describe the supported recovery path: %q", check.Hint)
		}
		return
	}
	t.Fatal("admin diagnostic check is missing")
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
