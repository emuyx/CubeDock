package m4399login

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type fake4399 struct {
	captchaNeeded bool
	loginSeen     url.Values
	captchaSeen   bool
}

func (f *fake4399) handler(t *testing.T, serverURL string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi/oauth-callback.html":
			if r.URL.Query().Get("gamekey") != "" {
				authURL := serverURL + "/oauth2/authorize.do?client_id=CLIENT&state=STATE&ref=REF&response_type=TOKEN&scope=basic"
				payload, _ := json.Marshal(map[string]any{"code": "0", "result": authURL})
				_, _ = w.Write(payload)
				return
			}
			payload, _ := json.Marshal(map[string]any{
				"code": "100", "message": "ok",
				"result": map[string]any{"uid": 42, "state": "STATE132"},
			})
			_, _ = w.Write(payload)
		case "/oauth2/authorize.do":
			_, _ = io.WriteString(w, `<html><body>
<input type="hidden" name="client_id" value="CLIENT"/>
<input type="hidden" name="state" value="STATE"/>
<input type="hidden" name="ref" value="REF"/>
<input type="hidden" name="response_type" value="TOKEN"/>
<input type="hidden" name="scope" value="basic"/>
<input type="hidden" name="bizId" value="2100001792"/>
<input type="hidden" name="redirect_uri" value="`+serverURL+`/openapi/oauth-callback.html?gamekey=44770"/>
<input type="hidden" name="auth_action" value="LOGIN"/>
<input type="hidden" name="sec" value="1"/>
</body></html>`)
		case "/oauth2/loginAndAuthorize.do":
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse form: %v", err)
			}
			f.loginSeen = r.PostForm
			if f.captchaNeeded && f.loginSeen.Get("captcha") != "abcd" {
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, `<p id="login_err_msg">验证码错误</p>`)
				return
			}
			w.Header().Set("Location", serverURL+"/openapi/oauth-callback.html?code=CODE&state=STATE132&uid=42")
			w.WriteHeader(http.StatusFound)
		case "/captcha.do":
			f.captchaSeen = true
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte{0x89, 0x50, 0x4e, 0x47})
		case "/x19/sdk/uni_sauth":
			inner, _ := json.Marshal(map[string]any{"oauth2": map[string]any{"access_token": "ACCESS42"}})
			payload, _ := json.Marshal(map[string]any{
				"status": "ok", "code": 200,
				"unisdk_login_json": base64.StdEncoding.EncodeToString(inner),
			})
			_, _ = w.Write(payload)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}
}

func newFakeService(t *testing.T, fake *fake4399) (*Service, *httptest.Server) {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.handler(t, server.URL)(w, r)
	}))
	service := New()
	service.HTTP = server.Client()
	service.CallbackURL = server.URL + "/openapi/oauth-callback.html?gamekey=44770&game_key=115716"
	service.LoginURL = server.URL + "/oauth2/loginAndAuthorize.do?channel=&sdk=op&sdk_version=3.18.0.682"
	service.UniSauthURL = server.URL + "/x19/sdk/uni_sauth"
	service.CaptchaURL = server.URL + "/captcha.do?captchaId="
	return service, server
}

func TestLogin(t *testing.T) {
	fake := &fake4399{}
	service, server := newFakeService(t, fake)
	defer server.Close()
	sauth, err := service.Login(context.Background(), "PlayerOne", "pw123", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if fake.loginSeen.Get("username") != "PlayerOne" || fake.loginSeen.Get("sec") != "1" ||
		fake.loginSeen.Get("auth_action") != "ORILOGIN" {
		t.Errorf("unexpected login form")
	}
	if fake.loginSeen.Get("password") == "pw123" || !strings.HasPrefix(fake.loginSeen.Get("password"), "U2FsdGVkX1") {
		t.Errorf("password was not AES-encrypted")
	}
	if decryptAES(fake.loginSeen.Get("password"), "lzYW5qaXVqa") != "pw123" {
		t.Errorf("AES password does not round-trip")
	}
	if sauth["sessionid"] != "STATE132" || sauth["sdkuid"] != "42" || sauth["access_token"] != "ACCESS42" {
		t.Errorf("unexpected sauth: %+v", sauth)
	}
	if sauth["app_channel"] != "4399com" || sauth["login_channel"] != "4399com" ||
		sauth["source_app_channel"] != "4399com" || sauth["platform"] != "ad" ||
		sauth["source_platform"] != "ad" || sauth["sdk_version"] != "3.18.0" {
		t.Errorf("unexpected channel fields: %+v", sauth)
	}
	for _, key := range []string{"udid", "deviceid", "client_login_sn"} {
		value, _ := sauth[key].(string)
		if len(value) != 32 || strings.Trim(value, "0123456789ABCDEF") != "" {
			t.Errorf("%s = %q", key, value)
		}
	}
}

func TestLoginCaptcha(t *testing.T) {
	fake := &fake4399{captchaNeeded: true}
	service, server := newFakeService(t, fake)
	defer server.Close()
	_, err := service.Login(context.Background(), "PlayerOne", "pw123", "", "")
	required, ok := err.(*CaptchaRequired)
	if !ok {
		t.Fatalf("error = %v, want CaptchaRequired", err)
	}
	if required.CaptchaID == "" || len(required.Image) != 4 || !fake.captchaSeen {
		t.Fatalf("unexpected captcha: %+v", required)
	}
	sauth, err := service.Login(context.Background(), "PlayerOne", "pw123", required.CaptchaID, "abcd")
	if err != nil {
		t.Fatal(err)
	}
	if fake.loginSeen.Get("captcha_id") != required.CaptchaID || fake.loginSeen.Get("captcha") != "abcd" {
		t.Errorf("captcha was not forwarded")
	}
	if sauth["sessionid"] != "STATE132" {
		t.Errorf("unexpected sauth")
	}
}

func decryptAES(encoded, passphrase string) string {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) < 16 || string(raw[:8]) != "Salted__" {
		return ""
	}
	salt := raw[8:16]
	var derived, previous []byte
	for len(derived) < 48 {
		sum := md5.Sum(append(append(append([]byte{}, previous...), []byte(passphrase)...), salt...))
		previous = sum[:]
		derived = append(derived, previous...)
	}
	block, err := aes.NewCipher(derived[:32])
	if err != nil {
		return ""
	}
	plain := make([]byte, len(raw)-16)
	cipher.NewCBCDecrypter(block, derived[32:48]).CryptBlocks(plain, raw[16:])
	if len(plain) == 0 {
		return ""
	}
	padding := int(plain[len(plain)-1])
	if padding <= 0 || padding > aes.BlockSize || padding > len(plain) {
		return ""
	}
	return string(plain[:len(plain)-padding])
}
