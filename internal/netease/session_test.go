package netease

import (
	"encoding/json"
	"strings"
	"testing"
)

const testInnerSession = `{"sdkuid":"42","sessionid":"session-token","udid":"udid","deviceid":"device","platform":"pc","app_channel":"a50_sdk_cn","access_token":"access"}`

const testMobileSession = `{"sdkuid":"42","sessionid":"mobile-session","udid":"udid","deviceid":"device","platform":"ad","app_channel":"netease.wyzymnqsd_cps_dev","login_channel":"netease","source_platform":"ad","source_app_channel":"netease.wyzymnqsd_cps_dev","gameid":"x19","sdk_version":"5.16.0","client_login_sn":"SN","access_token":"access"}`

func TestParseSessionAcceptsCookieAndDoubleEncoding(t *testing.T) {
	cookie, _ := json.Marshal(map[string]string{"sauth_json": testInnerSession})
	double, _ := json.Marshal(string(cookie))
	for name, input := range map[string]string{
		"cookie": string(cookie),
		"double": string(double),
		"inner":  testInnerSession,
	} {
		t.Run(name, func(t *testing.T) {
			session, err := ParseSession(input)
			if err != nil {
				t.Fatal(err)
			}
			if session.Method != AuthInterop {
				t.Fatalf("method = %q", session.Method)
			}
			if session.SDKUID != "42" || session.SessionID != "session-token" || session.AccessToken != "access" {
				t.Fatalf("session = %+v", session)
			}
		})
	}
}

func TestParseSessionInfersMobileMethod(t *testing.T) {
	session, err := ParseSession(testMobileSession)
	if err != nil {
		t.Fatal(err)
	}
	if session.Method != AuthMobileNetEase || session.Platform != "ad" ||
		session.AppChannel != "netease.wyzymnqsd_cps_dev" ||
		session.SDKVersion != "5.16.0" || session.ClientLoginSN != "SN" {
		t.Fatalf("mobile session = %+v", session)
	}
	if _, err := ParseSessionFor(AuthInterop, testMobileSession); err == nil {
		t.Fatal("mobile session accepted as interop")
	}
	if _, err := ParseSessionFor(AuthMobileNetEase, testInnerSession); err == nil {
		t.Fatal("interop session accepted as mobile")
	}
}

func TestParseSessionRejectsMissingFields(t *testing.T) {
	missing := strings.Replace(testInnerSession, `"udid":"udid",`, "", 1)
	if _, err := ParseSession(missing); err == nil || !strings.Contains(err.Error(), "udid") {
		t.Fatalf("missing field error = %v", err)
	}
}
