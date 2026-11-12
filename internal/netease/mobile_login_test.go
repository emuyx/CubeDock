package netease

import (
	"bytes"
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const testMobileDeviceKeyHex = "396a656339656a657675656479637738"

type mobileTestServer struct {
	configRequests   atomic.Int32
	methodRequests   atomic.Int32
	deviceRequests   atomic.Int32
	uploadRequests   atomic.Int32
	passwordRequests atomic.Int32
	uniSauthRequests atomic.Int32

	mu            sync.Mutex
	configQueries []string

	lastQuery   string
	lastParams  string
	lastPayload map[string]any
	deviceUDID  string

	verifyRequired bool
}

func (s *mobileTestServer) recordConfigQuery(query string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configQueries = append(s.configQueries, query)
}

func (s *mobileTestServer) configs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.configQueries...)
}

func newMobileTestServer(t *testing.T) (*httptest.Server, *mobileTestServer) {
	t.Helper()
	state := &mobileTestServer{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/config"):
			state.configRequests.Add(1)
			if got := r.Header.Get("User-Agent"); got != mobileMkeyUserAgent() {
				t.Errorf("config User-Agent = %q", got)
			}
			query := r.URL.Query()
			if query.Get("device_udid") == "" {
				t.Error("config query missing device_udid")
			}
			state.recordConfigQuery(r.URL.RawQuery)
			respondJSON(t, w, map[string]any{"ok": true})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/mpay/config/common.json"):
			if got := r.Header.Get("User-Agent"); got != mobileMkeyUserAgent() {
				t.Errorf("common config User-Agent = %q", got)
			}
			respondJSON(t, w, map[string]any{"ok": true})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/login_methods"):
			state.methodRequests.Add(1)
			if got := r.Header.Get("User-Agent"); got != mobileMkeyUserAgent() {
				t.Errorf("login_methods User-Agent = %q", got)
			}
			if r.URL.Query().Get("device_udid") == "" {
				t.Error("login_methods query missing device_udid")
			}
			respondJSON(t, w, map[string]any{"ok": true})
		case r.Method == http.MethodPost && r.URL.Path == "/mpay/api/devices/upload":
			state.uploadRequests.Add(1)
			keys := rawFormKeys(t, r)
			if len(keys) < 2 || keys[0] != "device_id" || keys[1] != "version" {
				t.Errorf("upload field order = %v", keys)
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse upload form: %v", err)
			}
			if got := r.Form.Get("device_id"); got != "dev-test" {
				t.Errorf("upload device_id = %q", got)
			}
			if got := r.Form.Get("version"); got != mobileGV {
				t.Errorf("upload version = %q", got)
			}
			if got := r.Form.Get("init_urs_device"); got != "" {
				t.Errorf("upload should not include init_urs_device, got %q", got)
			}
			if !strings.HasPrefix(r.Form.Get("transid"), r.Form.Get("udid")+"_") {
				t.Errorf("upload transid = %q, udid = %q", r.Form.Get("transid"), r.Form.Get("udid"))
			}
			if !strings.HasPrefix(r.Form.Get("mcount_transaction_id"), r.Form.Get("udid")+"_") {
				t.Errorf("upload mcount_transaction_id = %q", r.Form.Get("mcount_transaction_id"))
			}
			respondJSON(t, w, map[string]any{"upload_time": 1})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/devices"):
			state.deviceRequests.Add(1)
			if got := r.Header.Get("User-Agent"); got != mobileMkeyUserAgent() {
				t.Errorf("device User-Agent = %q", got)
			}
			keys := rawFormKeys(t, r)
			if len(keys) < 3 || keys[0] != "mac" || keys[2] != "init_urs_device" {
				t.Errorf("device field order = %v", keys)
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse device form: %v", err)
			}
			for _, key := range []string{"unique_id", "udid", "mac", "urs_udid", "ext_ci", "brand", "device_model", "resolution"} {
				if r.Form.Get(key) == "" {
					t.Errorf("device form missing %s", key)
				}
			}
			if got := r.Form.Get("init_urs_device"); got != "0" {
				t.Errorf("init_urs_device = %q, want 0", got)
			}
			state.deviceUDID = r.Form.Get("udid")
			if !strings.HasPrefix(r.Form.Get("transid"), state.deviceUDID+"_") {
				t.Errorf("registration transid = %q, udid = %q", r.Form.Get("transid"), state.deviceUDID)
			}
			respondJSON(t, w, map[string]any{
				"device": map[string]string{"id": "dev-test", "key": testMobileDeviceKeyHex},
			})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/devices/") && strings.HasSuffix(r.URL.Path, "/users"):
			state.passwordRequests.Add(1)
			state.lastQuery = r.URL.RawQuery
			keys := rawFormKeys(t, r)
			if len(keys) < 2 || keys[0] != "opt_fields" || keys[1] != "params" {
				t.Errorf("login field order = %v", keys)
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse login form: %v", err)
			}
			state.lastParams = r.Form.Get("params")
			if !strings.HasPrefix(r.Form.Get("transid"), state.deviceUDID+"_") {
				t.Errorf("login transid = %q, udid = %q", r.Form.Get("transid"), state.deviceUDID)
			}
			if state.verifyRequired {
				respondJSON(t, w, map[string]any{
					"code": 1351, "message": "需要安全验证",
					"verify_url": "https://example.com/verify?token=abc",
				})
				return
			}
			respondJSON(t, w, map[string]any{
				"code": 200,
				"user": map[string]string{"id": "uid-1", "token": "session-token-1"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/x19/sdk/uni_sauth":
			state.uniSauthRequests.Add(1)
			if got := r.Header.Get("User-Agent"); got != mobileDalvikUserAgent {
				t.Errorf("uni_sauth User-Agent = %q", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&state.lastPayload); err != nil {
				t.Errorf("decode uni_sauth payload: %v", err)
			}
			if got := state.lastPayload["sessionid"]; got != "session-token-1" {
				t.Errorf("uni_sauth sessionid = %v", got)
			}
			if got := state.lastPayload["deviceid"]; got != "dev-test" {
				t.Errorf("uni_sauth deviceid = %v", got)
			}
			if got := state.lastPayload["get_access_token"]; got != "1" {
				t.Errorf("uni_sauth get_access_token = %v", got)
			}
			if got := state.lastPayload["step"]; got != "0" {
				t.Errorf("uni_sauth step = %v", got)
			}
			if sdk, ok := state.lastPayload["sdklog"].(string); !ok || !strings.Contains(sdk, "device_model") {
				t.Errorf("uni_sauth sdklog = %v", state.lastPayload["sdklog"])
			}
			if aim, ok := state.lastPayload["aim_info"].(string); !ok || !strings.Contains(aim, "Asia/Shanghai") {
				t.Errorf("uni_sauth aim_info = %v", state.lastPayload["aim_info"])
			}
			loginJSON := base64.StdEncoding.EncodeToString([]byte(`{"oauth2":{"access_token":"access-token-1"}}`))
			respondJSON(t, w, map[string]any{
				"code": 200, "status": "ok", "unisdk_login_json": loginJSON,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, state
}

func rawFormKeys(t *testing.T, r *http.Request) []string {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read form body: %v", err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var keys []string
	for _, pair := range strings.Split(string(body), "&") {
		if pair == "" {
			continue
		}
		key, _, _ := strings.Cut(pair, "=")
		if decoded, err := url.QueryUnescape(key); err == nil {
			key = decoded
		}
		keys = append(keys, key)
	}
	return keys
}

func respondJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}

func newTestMobileService(t *testing.T, serverURL, dir string) *MobileLoginService {
	t.Helper()
	service := NewMobileLoginService(dir)
	service.BaseURL = serverURL
	service.UniSauthURL = serverURL + "/x19/sdk/uni_sauth"
	service.IPLookup = func(context.Context) string { return "127.0.0.1" }
	return service
}

func TestMobileLoginEndToEnd(t *testing.T) {
	server, state := newMobileTestServer(t)
	dir := t.TempDir()
	service := newTestMobileService(t, server.URL, dir)

	sauth, err := service.Login(context.Background(), "user@example.com", "secret-password")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got := sauth["platform"]; got != "ad" {
		t.Errorf("platform = %v", got)
	}
	if got := sauth["app_channel"]; got != mobileAppChannel {
		t.Errorf("app_channel = %v", got)
	}
	if got := sauth["sdkuid"]; got != "uid-1" {
		t.Errorf("sdkuid = %v", got)
	}
	if got := sauth["sessionid"]; got != "session-token-1" {
		t.Errorf("sessionid = %v", got)
	}
	if got := sauth["access_token"]; got != "access-token-1" {
		t.Errorf("access_token = %v", got)
	}
	if got := sauth["deviceid"]; got != "dev-test" {
		t.Errorf("deviceid = %v", got)
	}
	if got := sauth["udid"]; got == "" || got == nil {
		t.Error("udid is empty")
	}
	if state.configRequests.Load() != 1 || state.methodRequests.Load() != 1 ||
		state.deviceRequests.Load() != 1 || state.uploadRequests.Load() != 1 ||
		state.passwordRequests.Load() != 1 || state.uniSauthRequests.Load() != 1 {
		t.Fatalf("requests = config:%d methods:%d devices:%d upload:%d login:%d uni:%d",
			state.configRequests.Load(), state.methodRequests.Load(), state.deviceRequests.Load(),
			state.uploadRequests.Load(), state.passwordRequests.Load(), state.uniSauthRequests.Load())
	}
	configs := state.configs()
	if len(configs) != 1 {
		t.Fatalf("config queries = %d", len(configs))
	}
	firstConfig, err := url.ParseQuery(configs[0])
	if err != nil {
		t.Fatalf("parse config query: %v", err)
	}
	if firstConfig.Get("device_udid") == "" {
		t.Error("first config query missing device_udid")
	}
	if firstConfig.Get("device_id") != "" {
		t.Errorf("first config query should not have device_id, got %q", firstConfig.Get("device_id"))
	}
	query, err := url.ParseQuery(state.lastQuery)
	if err != nil {
		t.Fatalf("parse login query: %v", err)
	}
	if want := base64.StdEncoding.EncodeToString([]byte("user@example.com")); query.Get("un") != want {
		t.Errorf("login query un = %q, want %q", query.Get("un"), want)
	}
	payload, _ := decryptMobileParams(t, state.lastParams, testMobileDeviceKeyHex)
	if payload["username"] != "user@example.com" || payload["password"] != md5Hex(t, "secret-password") {
		t.Errorf("decrypted params = %#v", payload)
	}
	if payload["password_level"] != float64(3) {
		t.Errorf("password_level = %v", payload["password_level"])
	}
	if payload["unique_id"] == "" {
		t.Error("unique_id is empty")
	}

	// A second login must reuse the cached device identity and include device_id
	// in the preflight queries, but must not register or upload the device again.
	if _, err := service.Login(context.Background(), "user@example.com", "secret-password"); err != nil {
		t.Fatalf("second Login: %v", err)
	}
	if state.deviceRequests.Load() != 1 || state.uploadRequests.Load() != 1 {
		t.Fatalf("device registrations = %d, uploads = %d, want 1/1",
			state.deviceRequests.Load(), state.uploadRequests.Load())
	}
	if state.configRequests.Load() != 2 || state.methodRequests.Load() != 2 {
		t.Fatalf("preflight requests = config:%d methods:%d, want 2/2",
			state.configRequests.Load(), state.methodRequests.Load())
	}
	configs = state.configs()
	secondConfig, err := url.ParseQuery(configs[len(configs)-1])
	if err != nil {
		t.Fatalf("parse second config query: %v", err)
	}
	if secondConfig.Get("device_id") != "dev-test" {
		t.Errorf("second config device_id = %q", secondConfig.Get("device_id"))
	}
	if secondConfig.Get("device_udid") != firstConfig.Get("device_udid") {
		t.Error("device_udid changed between logins")
	}

	raw, err := os.ReadFile(filepath.Join(dir, mobileDeviceFile))
	if err != nil || len(raw) == 0 {
		t.Fatalf("device state not persisted: %v", err)
	}
}

func TestMobileLoginVerifyRequired(t *testing.T) {
	server, state := newMobileTestServer(t)
	state.verifyRequired = true
	service := newTestMobileService(t, server.URL, t.TempDir())

	_, err := service.Login(context.Background(), "user@example.com", "secret-password")
	var verify *MobileVerifyRequired
	if !errors.As(err, &verify) {
		t.Fatalf("error = %v, want MobileVerifyRequired", err)
	}
	if verify.URL != "https://example.com/verify?token=abc" {
		t.Fatalf("verify URL = %q", verify.URL)
	}
}

func TestEncryptMobileParamsRoundTrip(t *testing.T) {
	const uniqueID = "device-unique-id"
	const email = "user@example.com"
	const password = "secret-password"

	encoded, err := encryptMobileParams(testMobileDeviceKeyHex, uniqueID, email, password)
	if err != nil {
		t.Fatalf("encryptMobileParams: %v", err)
	}
	if strings.Contains(encoded, password) || strings.Contains(encoded, email) {
		t.Fatal("plaintext leaked into hex output")
	}
	payload, plain := decryptMobileParams(t, encoded, testMobileDeviceKeyHex)
	if payload["username"] != email || payload["password"] != md5Hex(t, password) || payload["unique_id"] != uniqueID {
		t.Fatalf("payload = %#v", payload)
	}
	if payload["password_level"] != float64(3) {
		t.Fatalf("password_level = %v", payload["password_level"])
	}
	if !strings.HasPrefix(plain, `{"username":`) {
		t.Fatalf("payload key order = %q", plain)
	}
	if strings.Contains(plain, password) {
		t.Fatal("plaintext password leaked into encrypted payload")
	}
}

func md5Hex(t *testing.T, value string) string {
	t.Helper()
	hash, err := mobilePasswordHash(value)
	if err != nil {
		t.Fatalf("mobilePasswordHash: %v", err)
	}
	return hash
}

func TestMobileDeviceIdentityGeneration(t *testing.T) {
	const androidID = "0123456789abcdef"
	if got, want := mobileURSUDID(androidID, mobileDeviceModel), "765f4f90207d95c4f407630461d04ec34d3ea944"; got != want {
		t.Fatalf("urs_udid = %q, want %q", got, want)
	}
	if got, want := mobileFingerprintMAC(androidID), "4f863c7987ec16840db0664203e76261"; got != want {
		t.Fatalf("mac = %q, want %q", got, want)
	}
	deviceUDID := mobileDeviceUDID(androidID)
	if len(deviceUDID) != 45 || deviceUDID[len(deviceUDID)-1] != '\t' {
		t.Fatalf("device_udid length/trailer = %d/%q", len(deviceUDID), deviceUDID[len(deviceUDID)-1])
	}
	if got, want := mobileFingerprintSHA256(), "876d1d1c8d175e2781d9267dcfebe970b740417f1e85edd5a9e5e61a6fb32626"; got != want {
		t.Fatalf("ext_ci = %q, want %q", got, want)
	}
	device := newMobileDevice()
	if device.UDID == "" || device.MAC != mobileFingerprintMAC(device.UDID) ||
		device.URSUDID != mobileURSUDID(device.UDID, mobileDeviceModel) ||
		device.DeviceUDID != mobileDeviceUDID(device.UDID) {
		t.Fatal("generated mobile device identity is not internally consistent")
	}
}

func TestMobilePasswordHashGBK(t *testing.T) {
	hash, err := mobilePasswordHash("密码")
	if err != nil {
		t.Fatalf("mobilePasswordHash: %v", err)
	}
	// MD5("密码".getBytes("GBK"))
	if hash != "d2293e30f0c964c5f6242486aa8a483c" {
		t.Fatalf("hash = %q", hash)
	}
}

func decryptMobileParams(t *testing.T, encoded, keyHex string) (map[string]any, string) {
	t.Helper()
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != 16 {
		t.Fatalf("invalid key: %v", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	plain := make([]byte, len(raw))
	for start := 0; start < len(raw); start += aes.BlockSize {
		block.Decrypt(plain[start:start+aes.BlockSize], raw[start:start+aes.BlockSize])
	}
	pad := int(plain[len(plain)-1])
	if pad == 0 || pad > aes.BlockSize || pad > len(plain) {
		t.Fatalf("invalid PKCS#7 padding %d", pad)
	}
	plain = plain[:len(plain)-pad]
	var payload map[string]any
	if err := json.Unmarshal(plain, &payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	return payload, string(plain)
}

func TestDecodeBase64JSON(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(`{"ok":true}`))
	decoded, err := decodeBase64JSON(encoded)
	if err != nil || string(decoded) != `{"ok":true}` {
		t.Fatalf("decodeBase64JSON = %q, %v", decoded, err)
	}
	direct, err := decodeBase64JSON(`{"ok":true}`)
	if err != nil || string(direct) != `{"ok":true}` {
		t.Fatalf("decodeBase64JSON direct = %q, %v", direct, err)
	}
	if _, err := decodeBase64JSON(""); err == nil {
		t.Fatal("empty value should fail")
	}
}
