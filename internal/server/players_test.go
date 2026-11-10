package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlayersEndpointEmpty(t *testing.T) {
	handler := newConsoleTestHandler(t)
	recorder := httptest.NewRecorder()
	handler.players(recorder, httptest.NewRequest(http.MethodGet, "/api/players", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var players []any
	if err := json.Unmarshal(recorder.Body.Bytes(), &players); err != nil {
		t.Fatal(err)
	}
	if len(players) != 0 {
		t.Fatalf("players = %+v", players)
	}
}

func TestPlayerActionsRejectBadInput(t *testing.T) {
	handler := newConsoleTestHandler(t)
	tests := []struct {
		name string
		body string
		call func(http.ResponseWriter, *http.Request)
	}{
		{"kick missing target", `{}`, handler.kickPlayer},
		{"kick unknown target", `{"target":"p99"}`, handler.kickPlayer},
		{"kick unknown field", `{"target":"p1","extra":1}`, handler.kickPlayer},
		{"transfer bad address", `{"target":"p1","address":"bad"}`, handler.transferPlayer},
		{"transfer missing address", `{"target":"p1"}`, handler.transferPlayer},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/player", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			test.call(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
