package server

import (
	"archive/zip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/core"
)

func newConsoleTestHandler(t *testing.T) *consoleHandler {
	t.Helper()
	manager, err := core.New(t.TempDir(), config.Defaults(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &consoleHandler{manager: manager}
}

func postConsole(handler *consoleHandler, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/api/console", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.runConsole(response, request)
	return response
}

func TestResourcePacksEndpoint(t *testing.T) {
	handler := newConsoleTestHandler(t)
	dir := t.TempDir()
	cfg := config.Defaults()
	cfg.LimboPacksDir = dir
	if err := handler.manager.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "test.mcpack")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte(`{"format_version":2,"header":{"name":"Test","uuid":"11111111-2222-3333-4444-555555555555","version":[1,0,0]}}`))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	handler.resourcePacks(recorder, httptest.NewRequest(http.MethodGet, "/api/packs", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var list core.ResourcePackList
	if err := json.Unmarshal(recorder.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if list.Dir != dir || len(list.Packs) != 1 || list.Packs[0].UUID != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("list = %+v", list)
	}
}

func TestRunConsoleEndpoint(t *testing.T) {
	handler := newConsoleTestHandler(t)

	tests := []struct {
		name     string
		body     string
		status   int
		contains string
	}{
		{"help", `{"command":"help"}`, http.StatusOK, "kick"},
		{"list", `{"command":"list"}`, http.StatusOK, "没有玩家"},
		{"unknown", `{"command":"bogus"}`, http.StatusBadRequest, "未知命令"},
		{"malformed", `{"command":`, http.StatusBadRequest, "请求数据无效"},
		{"unknown field", `{"command":"help","extra":1}`, http.StatusBadRequest, "请求数据无效"},
		{"too long", `{"command":"` + strings.Repeat("x", 513) + `"}`, http.StatusBadRequest, "过长"},
		{"kick missing", `{"command":"kick p1"}`, http.StatusBadRequest, "未找到玩家"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := postConsole(handler, test.body)
			if response.Code != test.status {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("body %q missing %q", response.Body.String(), test.contains)
			}
		})
	}
}
