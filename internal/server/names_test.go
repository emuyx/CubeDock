package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/emuyx/cubedock/internal/core"
)

func TestSetAccountNameRejectsStaleIdentity(t *testing.T) {
	handler := newConsoleTestHandler(t)
	if _, err := handler.manager.AddAccount("name-token", core.AccountMeta{
		Name: "Alice", Email: "a@example.com", UID: "1",
		AuthMethod: "interop", Platform: "interop",
	}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/account/set-name",
		strings.NewReader(`{"index":0,"name":"ValidName","uid":"2","email":"b@example.com"}`))
	recorder := httptest.NewRecorder()
	handler.setAccountName(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("stale identity status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestSetAccountNameValidation(t *testing.T) {
	handler := newConsoleTestHandler(t)
	tests := []struct {
		name string
		body string
	}{
		{"empty name", `{"index":0,"name":""}`},
		{"unknown field", `{"index":0,"name":"ValidName","extra":1}`},
		{"missing account", `{"index":1,"name":"ValidName"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/account/set-name", strings.NewReader(test.body))
			recorder := httptest.NewRecorder()
			handler.setAccountName(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
