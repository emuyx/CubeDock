package friends

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRequestSignatureVectors(t *testing.T) {
	for _, vector := range []struct {
		token, path, body, gateway, core string
	}{
		{"fixture-token", "/user-addme-friends/", "{}", "NClpuGGtdPsp666u1", "CKjURyMQiwSo1yoQ1"},
		{"other-fixture", "/user-reply-friend/", `{"fid":123456789,"accept":true,"accept_ui":"联系人-好友申请"}`, "u3hsKb7rLzY1uijv1", "h4evVwMoE0iIByjT1"},
	} {
		key := md5.Sum([]byte(vector.token))
		if got := requestSignature(key[:], vector.path, []byte(vector.body), false); got != vector.gateway {
			t.Fatalf("gateway signature = %s, want %s", got, vector.gateway)
		}
		if got := requestSignature(key[:], vector.path, []byte(vector.body), true); got != vector.core {
			t.Fatalf("core signature = %s, want %s", got, vector.core)
		}
	}
}

func TestPollAcceptsPendingDeduplicatesAndRetries(t *testing.T) {
	key := md5.Sum([]byte("fixture-token"))
	list := `[{"uid":"9007199254740993","nickname":"Alice\n","apply_time":10},{"uid":42,"nickname":"self"},{"uid":0},{"uid":"bad"},{"uid":9007199254740993,"apply_time":10}]`
	var attempts []uint64
	var events []string
	shouldFail := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || r.Header.Get("user-id") != "42" ||
			r.Header.Get("user-token") != requestSignature(key[:], r.URL.Path, body, false) {
			t.Error("missing or incorrect authenticated request")
		}
		switch r.URL.Path {
		case "/user-addme-friends/":
			if string(body) != "{}" {
				t.Error("incorrect list body")
			}
			_, _ = io.WriteString(w, `{"code":0,"entities":`+list+`}`)
		case "/user-reply-friend/":
			var reply struct {
				FID      uint64 `json:"fid"`
				Accept   bool   `json:"accept"`
				AcceptUI string `json:"accept_ui"`
			}
			if json.Unmarshal(body, &reply) != nil || !reply.Accept || reply.AcceptUI != "联系人-好友申请" {
				t.Error("incorrect acceptance payload")
			}
			attempts = append(attempts, reply.FID)
			if shouldFail {
				_, _ = io.WriteString(w, `{"code":123,"message":"private-test-secret"}`)
			} else {
				_, _ = io.WriteString(w, `{"code":0}`)
			}
		default:
			t.Error("unexpected endpoint")
		}
	}))
	defer server.Close()

	client := &gatewayClient{http: server.Client(), gateway: server.URL, uid: "42", key: key[:]}
	watcher := &watcher{client: client, accepted: map[uint64]string{}, event: func(line string) { events = append(events, line) }}
	ctx := context.Background()

	if err := watcher.poll(ctx); err == nil || strings.Contains(err.Error(), "secret") || len(events) != 0 {
		t.Fatal("business failure accepted or leaked the response")
	}
	shouldFail = false
	for range 2 {
		if err := watcher.poll(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(attempts, []uint64{9007199254740993, 9007199254740993}) ||
		len(events) != 1 || strings.Contains(events[0], "\n") {
		t.Fatalf("retries/deduplication/events = %v %v", attempts, events)
	}

	list = `[]`
	if err := watcher.poll(ctx); err != nil || len(watcher.accepted) != 0 {
		t.Fatal("cache not pruned")
	}
	list = `[{"uid":9007199254740993,"apply_time":11}]`
	if err := watcher.poll(ctx); err != nil || len(attempts) != 3 {
		t.Fatal("renewed application suppressed")
	}
}

func TestPollContinuesAfterOneAcceptFailure(t *testing.T) {
	key := md5.Sum([]byte("fixture-token"))
	list := `[{"uid":101,"nickname":"bad","apply_time":1},{"uid":102,"nickname":"good","apply_time":1}]`
	var attempts []uint64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user-addme-friends/" {
			_, _ = io.WriteString(w, `{"code":0,"entities":`+list+`}`)
			return
		}
		var reply struct {
			FID uint64 `json:"fid"`
		}
		body, _ := io.ReadAll(r.Body)
		if json.Unmarshal(body, &reply) != nil {
			t.Error("invalid acceptance body")
		}
		attempts = append(attempts, reply.FID)
		if reply.FID == 101 {
			_, _ = io.WriteString(w, `{"code":123}`)
			return
		}
		_, _ = io.WriteString(w, `{"code":0}`)
	}))
	defer server.Close()

	watcher := &watcher{
		client:   &gatewayClient{http: server.Client(), gateway: server.URL, uid: "42", key: key[:]},
		accepted: map[uint64]string{},
		event:    func(string) {},
	}
	err := watcher.poll(context.Background())
	if err == nil || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("failure = %v", err)
	}
	if !reflect.DeepEqual(attempts, []uint64{101, 102}) {
		t.Fatalf("attempts = %v", attempts)
	}
	if _, ok := watcher.accepted[102]; !ok {
		t.Fatal("good request was not accepted")
	}
	if _, ok := watcher.accepted[101]; ok {
		t.Fatal("failed request was cached as accepted")
	}
}

func TestPollRediscoversStaleGateway(t *testing.T) {
	key := md5.Sum([]byte("fixture-token"))
	goodHits, oldHits := 0, 0
	good := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		goodHits++
		_, _ = io.WriteString(w, `{"code":0,"entities":[]}`)
	}))
	defer good.Close()
	old := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		oldHits++
		_, _ = io.WriteString(w, `{"code":0,"entities":[]}`)
	}))
	defer old.Close()
	manifest := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"ApiGatewayUrl":"`+good.URL+`"}`)
	}))
	defer manifest.Close()

	httpClient := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // test-only fixtures
	}}
	client := &gatewayClient{
		http:      httpClient,
		manifest:  manifest.URL,
		gateway:   old.URL,
		gatewayAt: time.Now().Add(-gatewayRefresh - time.Minute),
		uid:       "42",
		key:       key[:],
	}
	watcher := &watcher{client: client, accepted: map[uint64]string{}, event: func(string) {}}
	if err := watcher.poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if oldHits != 0 || goodHits != 1 || client.gateway != good.URL {
		t.Fatalf("old=%d good=%d gateway=%q", oldHits, goodHits, client.gateway)
	}
}

func TestPollClearsGatewayOnListFailure(t *testing.T) {
	key := md5.Sum([]byte("fixture-token"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	client := &gatewayClient{http: server.Client(), gateway: server.URL, gatewayAt: time.Now(), uid: "42", key: key[:]}
	watcher := &watcher{client: client, accepted: map[uint64]string{}, event: func(string) {}}
	if err := watcher.poll(context.Background()); err == nil {
		t.Fatal("failed list was accepted")
	}
	if client.gateway != "" {
		t.Fatalf("gateway was not cleared: %q", client.gateway)
	}
}

func TestPendingRenegotiatesSignatureAfterFormatChange(t *testing.T) {
	key := md5.Sum([]byte("fixture-token"))
	requireCore := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("user-token") != requestSignature(key[:], r.URL.Path, body, requireCore) {
			_, _ = io.WriteString(w, `{"code":22}`)
			return
		}
		_, _ = io.WriteString(w, `{"code":0,"entities":[]}`)
	}))
	defer server.Close()

	client := &gatewayClient{http: server.Client(), gateway: server.URL, uid: "42", key: key[:]}
	for _, step := range []struct {
		requireCore bool
		wantCore    bool
	}{
		{false, false},
		{true, true},
		{false, false},
	} {
		requireCore = step.requireCore
		if _, err := client.pending(context.Background()); err != nil {
			t.Fatalf("requireCore=%t: %v", requireCore, err)
		}
		if client.coreSignature != step.wantCore {
			t.Fatalf("coreSignature = %t, want %t", client.coreSignature, step.wantCore)
		}
	}
}

func TestPollIsBoundedAndCancellable(t *testing.T) {
	var applicants []map[string]any
	for i := range 25 {
		applicants = append(applicants, map[string]any{"uid": i + 100})
	}
	accepted := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user-addme-friends/" {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "entities": applicants})
			return
		}
		accepted++
		_, _ = io.WriteString(w, `{"code":0}`)
	}))
	defer server.Close()

	watcher := &watcher{
		client:   &gatewayClient{http: server.Client(), gateway: server.URL, uid: "42", key: make([]byte, 16)},
		accepted: map[uint64]string{},
		event:    func(string) {},
	}
	if err := watcher.poll(context.Background()); err != nil || accepted != 20 {
		t.Fatalf("batch limit = %d, %v", accepted, err)
	}
	if err := watcher.poll(context.Background()); err != nil || accepted != 25 {
		t.Fatal("remaining requests not handled")
	}

	watcher.accepted = map[uint64]string{}
	watcher.pace = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	watcher.event = func(string) { cancel() }
	if err := watcher.poll(ctx); err != context.Canceled || accepted != 26 {
		t.Fatalf("cancellation = %d, %v", accepted, err)
	}
	if sleep(ctx, time.Hour) {
		t.Fatal("cancelled wait completed normally")
	}
}

func TestBadResponsesAreRejectedWithoutLeaking(t *testing.T) {
	for _, body := range []string{
		`{}`, `null`, `{"code":1,"message":"private-test-secret"}`, `{"code":0,"entities":[`,
		strings.Repeat("x", (1<<20)+1),
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, body)
		}))
		client := &gatewayClient{http: server.Client(), gateway: server.URL, uid: "42", key: make([]byte, 16)}
		if _, err := client.pending(context.Background()); err == nil || strings.Contains(err.Error(), "secret") {
			server.Close()
			t.Fatal("bad response accepted or leaked")
		}
		server.Close()
	}
}

func TestDiscoverAndRedirectPolicy(t *testing.T) {
	manifest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("user-token") != "" || r.Header.Get("user-id") != "" {
			t.Error("metadata request carried credentials")
		}
		_, _ = io.WriteString(w, `{"ApiGatewayUrl":"https://gateway.example/"}`)
	}))
	defer manifest.Close()
	client := &gatewayClient{http: manifest.Client(), manifest: manifest.URL}
	if err := client.discover(context.Background()); err != nil || client.gateway != "https://gateway.example" {
		t.Fatalf("discovery failed: %v", err)
	}

	reached := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	client.http = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	client.gateway, client.uid, client.key = redirect.URL, "42", make([]byte, 16)
	if _, err := client.pending(context.Background()); err == nil || reached {
		t.Fatal("followed a credential redirect")
	}
}

func TestPollCancellationInterruptsHTTP(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	client := &gatewayClient{http: server.Client(), gateway: server.URL, uid: "42", key: make([]byte, 16)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := client.pending(ctx); done <- err }()
	<-started
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled HTTP request succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP cancellation blocked")
	}
}
