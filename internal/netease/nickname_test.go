package netease

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateNickname(t *testing.T) {
	valid := []string{"星海", "星海旅", "方块小匠", "abc", "PixelMiner4", strings.Repeat("星", 8)}
	for _, name := range valid {
		if err := ValidateNickname(name); err != nil {
			t.Errorf("ValidateNickname(%q) = %v", name, err)
		}
	}
	invalid := []string{"", "   ", "a", "ab", strings.Repeat("星", 9), strings.Repeat("a", 13), "bad\nname"}
	for _, name := range invalid {
		if err := ValidateNickname(name); err == nil {
			t.Errorf("ValidateNickname(%q) = nil, want error", name)
		}
	}
}

type nicknameRequest struct {
	path    string
	headers http.Header
	body    map[string]string
}

func TestSetNickname(t *testing.T) {
	requests := make(chan nicknameRequest, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		requests <- nicknameRequest{path: r.URL.Path, headers: r.Header.Clone(), body: body}
		_, _ = w.Write([]byte(`{"code":0,"message":"ok"}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.g79 = ReleaseInfo{APIGatewayURL: server.URL}
	client.creds = Credentials{UserID: "42", Token: "token-value"}
	if err := client.SetNickname(context.Background(), " 星海旅人 "); err != nil {
		t.Fatal(err)
	}
	request := <-requests
	if request.path != "/pe-nickname-setting/update" {
		t.Errorf("path = %q", request.path)
	}
	if request.headers.Get("user-id") != "42" || request.headers.Get("user-token") == "" {
		t.Errorf("missing auth headers: %v", request.headers)
	}
	if request.body["name"] != "星海旅人" {
		t.Errorf("name = %q", request.body["name"])
	}
}

func TestSetNicknameErrorCode(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":7,"message":"名称不可用"}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.g79 = ReleaseInfo{APIGatewayURL: server.URL}
	client.creds = Credentials{UserID: "42", Token: "token-value"}
	err := client.SetNickname(context.Background(), "星海旅人")
	if err == nil || !strings.Contains(err.Error(), "名称不可用") {
		t.Fatalf("error = %v", err)
	}
}

func TestSetNicknameKeepsCooldownMessage(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":18,"message":"昵称修改后30天内不能再次修改"}`))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	client.g79 = ReleaseInfo{APIGatewayURL: server.URL}
	client.creds = Credentials{UserID: "42", Token: "token-value"}
	err := client.SetNickname(context.Background(), "星海旅人")
	if err == nil || !strings.Contains(err.Error(), "30天") {
		t.Fatalf("error = %v", err)
	}
}
