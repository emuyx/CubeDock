package netease

import (
	"encoding/json"
	"strings"
	"testing"
)

const testInnerSession = `{"sdkuid":"42","sessionid":"session-token","udid":"udid","deviceid":"device","platform":"pc","app_channel":"a50_sdk_cn","access_token":"access"}`

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
			if session.SDKUID != "42" || session.SessionID != "session-token" || session.AccessToken != "access" {
				t.Fatalf("session = %+v", session)
			}
		})
	}
}

func TestParseSessionRejectsPhoneAndMissingFields(t *testing.T) {
	phone := strings.Replace(testInnerSession, `"platform":"pc"`, `"platform":"ad"`, 1)
	phone = strings.Replace(phone, `"app_channel":"a50_sdk_cn"`, `"app_channel":"netease"`, 1)
	if _, err := ParseSession(phone); err == nil || !strings.Contains(err.Error(), "普通手游") {
		t.Fatalf("phone session error = %v", err)
	}
	missing := strings.Replace(testInnerSession, `"udid":"udid",`, "", 1)
	if _, err := ParseSession(missing); err == nil || !strings.Contains(err.Error(), "udid") {
		t.Fatalf("missing field error = %v", err)
	}
}
