package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
