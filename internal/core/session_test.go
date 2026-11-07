package core

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func fakeSession(platform, channel string) map[string]any {
	expiry := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":1893456000}`))
	return map[string]any{
		"access_token": "header." + expiry + ".signature",
		"app_channel":  channel,
		"deviceid":     "test-device",
		"gameid":       "x19",
		"platform":     platform,
		"sdk_version":  "4.16.0",
		"sdkuid":       "test-uid",
		"sessionid":    "1-test-session",
		"udid":         "ABCDEF0123456789",
	}
}

func TestNormalizeInteropSessionShapes(t *testing.T) {
	inner := fakeSession("pc", "a50_sdk_cn")
	innerJSON, _ := json.Marshal(inner)
	cookie, _ := json.Marshal(map[string]string{"sauth_json": string(innerJSON)})
	wrapped, _ := json.Marshal(string(cookie))

	for name, input := range map[string]string{
		"inner": string(innerJSON), "cookie": string(cookie), "json string": string(wrapped),
	} {
		token, platform, expiresAt, err := NormalizeInteropSession(input)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if platform != "interop" || !strings.HasPrefix(token, `{"sauth_json":`) {
			t.Fatalf("%s: platform=%q token=%q", name, platform, token)
		}
		if expiresAt != time.Unix(1893456000, 0).Format(time.RFC3339) {
			t.Fatalf("%s: expiry=%q", name, expiresAt)
		}
	}
}

func TestNormalizeInteropSessionRejectsMobileAndMissingFields(t *testing.T) {
	mobile, _ := json.Marshal(fakeSession("ad", "netease"))
	if _, _, _, err := NormalizeInteropSession(string(mobile)); err == nil {
		t.Fatal("mobile session accepted")
	}
	broken := fakeSession("pc", "a50_sdk_cn")
	delete(broken, "sessionid")
	raw, _ := json.Marshal(broken)
	if _, _, _, err := NormalizeInteropSession(string(raw)); err == nil || !strings.Contains(err.Error(), "sessionid") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, _, err := NormalizeInteropSession(""); err == nil {
		t.Fatal("empty session accepted")
	}
}
