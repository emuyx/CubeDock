package netease

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProfileNameMissingReturnsPartialIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"entity":{"entity_id":"42","name":"","level":1}}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.g79 = ReleaseInfo{CoreServerURL: server.URL}
	client.creds = Credentials{UserID: "42", Token: "token-value"}
	profile, err := client.Profile(context.Background())
	if !errors.Is(err, ErrProfileNameMissing) {
		t.Fatalf("error = %v, want ErrProfileNameMissing", err)
	}
	if profile.UID != "42" || profile.Level != 1 {
		t.Fatalf("partial profile = %+v", profile)
	}
}

func TestPEAuthRateLimitDetection(t *testing.T) {
	if !IsPEAuthRateLimited(&PEAuthError{Code: 2100, Message: "busy"}) {
		t.Fatal("code 2100 should be rate limited")
	}
	if IsPEAuthRateLimited(&PEAuthError{Code: 32, Message: "maintenance"}) {
		t.Fatal("code 32 should not be rate limited")
	}
}
