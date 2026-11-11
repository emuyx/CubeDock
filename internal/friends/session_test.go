package friends

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionPendingAndAccept(t *testing.T) {
	var accepted uint64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user-addme-friends/":
			_, _ = w.Write([]byte(`{"code":0,"entities":[{"uid":7,"nickname":"Alice","apply_time":1},{"uid":42,"nickname":"self"}]}`))
		case "/user-reply-friend/":
			var body struct {
				FID    uint64 `json:"fid"`
				Accept bool   `json:"accept"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if !body.Accept {
				t.Errorf("accept = false")
			}
			accepted = body.FID
			_, _ = w.Write([]byte(`{"code":0}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	key := make([]byte, 32)
	for index := range key {
		key[index] = byte(index + 1)
	}
	session, err := NewSession(42, key)
	if err != nil {
		t.Fatal(err)
	}
	session.client.http = server.Client()
	session.client.gateway = server.URL

	requests, err := session.Pending(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].UID != 7 || requests[0].Name != "Alice" {
		t.Fatalf("requests = %+v", requests)
	}
	if err := session.Accept(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if accepted != 7 {
		t.Fatalf("accepted = %d", accepted)
	}
}

func TestNewSessionRejectsInvalidInput(t *testing.T) {
	if _, err := NewSession(0, make([]byte, 32)); err == nil {
		t.Fatal("zero UID accepted")
	}
	if _, err := NewSession(42, make([]byte, 8)); err == nil {
		t.Fatal("short encryption key accepted")
	}
	if _, err := NewSession(42, make([]byte, 16)); err != nil {
		t.Fatalf("16-byte key rejected: %v", err)
	}
}
