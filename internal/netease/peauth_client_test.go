package netease

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticatePerformsPEAuth(t *testing.T) {
	var requestSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pe-authentication" {
			http.NotFound(w, r)
			return
		}
		requestSeen = true
		body, _ := io.ReadAll(r.Body)
		plain, err := decryptEnvelope(string(body))
		if err != nil {
			t.Errorf("request envelope: %v", err)
			return
		}
		var payload peAuthRequest
		if err := json.Unmarshal(plain, &payload); err != nil {
			t.Errorf("request json: %v", err)
			return
		}
		if payload.EngineVersion != interopEngineVersion || payload.Sign == "" || payload.Seed == "" {
			t.Errorf("request payload = %+v", payload)
		}
		response, _ := encryptEnvelope([]byte(`{"code":0,"message":"ok","entity":{"entity_id":"42","token":"0123456789abcdef","seed":"seed-value"}}`), rand.Reader)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.g79 = ReleaseInfo{CoreServerURL: server.URL}
	client.discovered = true
	if err := client.Authenticate(context.Background(), testInnerSession); err != nil {
		t.Fatal(err)
	}
	if !requestSeen {
		t.Fatal("PE request was not sent")
	}
	creds := client.Credentials()
	if creds.UserID != "42" || creds.Token != "0123456789abcdef" || creds.Seed != "seed-value" {
		t.Fatalf("credentials = %+v", creds)
	}
}

func TestProfileAndCreateRoom(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pe-user-detail/get":
			if r.Header.Get("user-token") == "" {
				t.Error("user-token header missing")
			}
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","entity":{"entity_id":"42","name":"Tester","level":9}}`))
		case "/transfer":
			_, _ = w.Write([]byte(`[{"status":1,"ip":"127.0.0.1","id":7,"SignalWebPort":18080,"ports":[19132,19133]}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.g79 = ReleaseInfo{CoreServerURL: server.URL, TransferServerURL: server.URL + "/transfer"}
	client.discovered = true
	client.creds = Credentials{UserID: "42", Token: "0123456789abcdef", Session: Session{SDKUID: "42"}}
	profile, err := client.Profile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if profile.Name != "Tester" || profile.UID != "42" || profile.Level != 9 {
		t.Fatalf("profile = %+v", profile)
	}
	auth, err := client.CreateRoom(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if auth.PlayerName != "Tester" || auth.UserID != 42 {
		t.Fatalf("room auth = %+v", auth)
	}
	if err := ValidateRoomAuth(auth); err != nil {
		t.Fatal(err)
	}
}

func TestEnvelopeHexRoundTripThroughServer(t *testing.T) {
	sealed, err := encryptEnvelope([]byte("hello"), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hex.DecodeString(sealed); err != nil {
		t.Fatal(err)
	}
	opened, err := decryptEnvelope(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if string(opened) != "hello" {
		t.Fatalf("opened = %q", opened)
	}
}
