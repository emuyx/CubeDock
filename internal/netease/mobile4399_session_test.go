package netease

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestProfileFor4399(t *testing.T) {
	profile, err := profileFor(AuthMobile4399)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Platform != "ad" || profile.AppChannel != "4399com" ||
		profile.LoginChannel != "4399com" || profile.SourceAppChannel != "4399com" ||
		profile.SourcePlatform != "ad" || profile.SDKVersion != "3.18.0" ||
		profile.PayChannel != "4399com" {
		t.Errorf("unexpected 4399 profile: %+v", profile)
	}
	if profile.SignSP != mobileNetEasePEAuthProfile.SignSP ||
		profile.SignTR != mobileNetEasePEAuthProfile.SignTR {
		t.Errorf("unexpected sign parameters")
	}
}

func TestNormalize4399SessionFillsChannelDefaults(t *testing.T) {
	inner := map[string]any{
		"sdkuid": "42", "sessionid": "SESSION", "udid": "UDID", "deviceid": "DEV",
		"access_token": "AT", "gameid": "x19",
	}
	raw, _ := json.Marshal(inner)
	token, method, _, err := NormalizeSession(string(raw), AuthMobile4399)
	if err != nil {
		t.Fatal(err)
	}
	if method != AuthMobile4399 {
		t.Fatalf("method = %q", method)
	}
	var outer struct {
		SauthJSON string `json:"sauth_json"`
	}
	if err := json.Unmarshal([]byte(token), &outer); err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(outer.SauthJSON), &parsed); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"app_channel": "4399com", "login_channel": "4399com",
		"platform": "ad", "sdk_version": "3.18.0",
		"source_app_channel": "4399com", "source_platform": "ad",
	} {
		if got := strings.TrimSpace(fmt.Sprint(parsed[key])); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}
