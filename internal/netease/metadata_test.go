package netease

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverLoadsReleaseDocuments(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/g79":
			_, _ = w.Write([]byte(`{"CoreServerUrl":"https://core.example","ApiGatewayUrl":"https://gateway.example","TransferServerUrl":"https://transfer.example"}`))
		case "/x19":
			_, _ = w.Write([]byte(`{"ApiGatewayGrayUrl":"https://gray.example"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.G79URL = server.URL + "/g79"
	client.X19URL = server.URL + "/x19"
	if err := client.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	release := client.Release()
	if release.CoreServerURL != "https://core.example" || release.TransferServerURL != "https://transfer.example" {
		t.Fatalf("release = %+v", release)
	}
	if client.GrayGateway() != "https://gray.example" {
		t.Fatalf("gray gateway = %q", client.GrayGateway())
	}
}

func TestDiscoverRejectsNonHTTPS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/g79" {
			_, _ = w.Write([]byte(`{"CoreServerUrl":"http://insecure.example"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.G79URL = server.URL + "/g79"
	client.X19URL = server.URL + "/x19"
	if err := client.Discover(context.Background()); err == nil {
		t.Fatal("insecure service URL accepted")
	}
}

func TestAuthenticateRequiresCaptures(t *testing.T) {
	client := NewClient(nil)
	if err := client.Authenticate(context.Background(), testInnerSession); err != ErrAwaitingCaptures {
		t.Fatalf("Authenticate error = %v", err)
	}
	if client.Credentials().Session.SDKUID != "42" {
		t.Fatalf("credentials = %+v", client.Credentials())
	}
}
