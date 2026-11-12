package netease

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ParseSession accepts the normalized sauth forms used by the console:
// {"sauth_json":"<inner>"}, a double-encoded variant, or the inner object.
// The auth method is inferred from platform/channel.
func ParseSession(input string) (Session, error) {
	return ParseSessionFor("", input)
}

// ParseSessionFor parses a session for a specific auth method. An empty method
// keeps backwards compatibility and infers it from the session fields.
func ParseSessionFor(method AuthMethod, input string) (Session, error) {
	inner, err := parseSessionInner(input)
	if err != nil {
		return Session{}, err
	}
	session := Session{
		SDKUID:           textField(inner, "sdkuid"),
		SessionID:        textField(inner, "sessionid"),
		UDID:             textField(inner, "udid"),
		DeviceID:         textField(inner, "deviceid"),
		Platform:         strings.ToLower(textField(inner, "platform")),
		AppChannel:       textField(inner, "app_channel"),
		LoginChannel:     textField(inner, "login_channel"),
		SourcePlatform:   strings.ToLower(textField(inner, "source_platform")),
		SourceAppChannel: textField(inner, "source_app_channel"),
		GameID:           textField(inner, "gameid"),
		SDKVersion:       textField(inner, "sdk_version"),
		ClientLoginSN:    textField(inner, "client_login_sn"),
		AccessToken:      textField(inner, "access_token"),
		Raw:              inner,
	}
	for _, field := range []string{"sdkuid", "sessionid", "udid", "deviceid"} {
		if textField(inner, field) == "" {
			return Session{}, fmt.Errorf("会话缺少字段 %s", field)
		}
	}

	if strings.TrimSpace(string(method)) == "" {
		method = inferAuthMethod(session)
	}
	switch method {
	case AuthInterop:
		if session.Platform != "pc" && session.AppChannel != "a50_sdk_cn" {
			label := session.Platform
			if label == "" || label == "<nil>" {
				label = session.AppChannel
			}
			if label == "" || label == "<nil>" {
				label = "ad"
			}
			return Session{}, fmt.Errorf("这是普通手游会话（%s），当前选择的是基岩互通版登录方式", label)
		}
		if session.AppChannel == "" {
			session.AppChannel = "a50_sdk_cn"
		}
		if session.Platform == "" {
			session.Platform = "pc"
		}
	case AuthMobileNetEase:
		if session.Platform == "pc" || session.AppChannel == "a50_sdk_cn" {
			return Session{}, fmt.Errorf("这是基岩互通版会话，请选择互通版登录方式")
		}
		if session.Platform == "" {
			session.Platform = "ad"
		}
		if session.AccessToken == "" {
			return Session{}, fmt.Errorf("手游会话缺少 access_token")
		}
	case AuthMobile4399:
		if session.Platform == "" {
			session.Platform = "ad"
		}
	default:
		return Session{}, fmt.Errorf("未知的登录方式 %q", method)
	}
	session.Method = method
	return session, nil
}

func inferAuthMethod(session Session) AuthMethod {
	channel := strings.ToLower(session.AppChannel + " " + session.SourceAppChannel + " " + session.LoginChannel)
	if strings.Contains(channel, "4399") {
		return AuthMobile4399
	}
	if session.Platform == "pc" || session.AppChannel == "a50_sdk_cn" {
		return AuthInterop
	}
	if session.Platform == "ad" || strings.Contains(channel, "netease") {
		return AuthMobileNetEase
	}
	return ""
}

func parseSessionInner(input string) (map[string]any, error) {
	var value any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &value); err != nil {
		return nil, fmt.Errorf("会话 JSON 格式无效")
	}
	// Launcher cookies are commonly JSON-encoded more than once.
	for range 3 {
		text, ok := value.(string)
		if !ok {
			break
		}
		value = nil
		if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &value); err != nil {
			return nil, fmt.Errorf("会话 JSON 格式无效")
		}
	}
	outer, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("会话 JSON 必须是对象")
	}
	inner := outer
	if raw, exists := outer["sauth_json"]; exists {
		switch typed := raw.(type) {
		case string:
			var parsed map[string]any
			if json.Unmarshal([]byte(strings.TrimSpace(typed)), &parsed) != nil {
				return nil, fmt.Errorf("sauth_json 字段不是有效的 JSON 对象")
			}
			inner = parsed
		case map[string]any:
			inner = typed
		default:
			return nil, fmt.Errorf("sauth_json 字段类型无效")
		}
	}
	return inner, nil
}

// NormalizeSession validates a session for one provider and returns the token
// form stored by CubeDock, the resolved method and the access-token expiry.
func NormalizeSession(input string, method AuthMethod) (token string, resolved AuthMethod, expiresAt string, err error) {
	inner, err := parseSessionInner(input)
	if err != nil {
		return "", "", "", err
	}
	session, err := ParseSessionFor(method, input)
	if err != nil {
		return "", "", "", err
	}
	resolved = session.Method
	if resolved == AuthInterop {
		if textField(inner, "app_channel") == "" {
			inner["app_channel"] = "a50_sdk_cn"
		}
		if textField(inner, "platform") == "" {
			inner["platform"] = "pc"
		}
	}
	if resolved == AuthMobileNetEase {
		profile, profileErr := profileFor(resolved)
		if profileErr == nil {
			if textField(inner, "platform") == "" {
				inner["platform"] = profile.Platform
			}
			if textField(inner, "app_channel") == "" {
				inner["app_channel"] = profile.AppChannel
			}
			if textField(inner, "login_channel") == "" {
				inner["login_channel"] = profile.LoginChannel
			}
			if textField(inner, "source_platform") == "" {
				inner["source_platform"] = profile.SourcePlatform
			}
			if textField(inner, "source_app_channel") == "" {
				inner["source_app_channel"] = profile.SourceAppChannel
			}
			if textField(inner, "sdk_version") == "" {
				inner["sdk_version"] = profile.SDKVersion
			}
		}
	}
	compact, err := json.Marshal(inner)
	if err != nil {
		return "", "", "", fmt.Errorf("序列化会话失败：%w", err)
	}
	outerJSON, err := json.Marshal(map[string]string{"sauth_json": string(compact)})
	if err != nil {
		return "", "", "", fmt.Errorf("序列化 Cookie 失败：%w", err)
	}
	return string(outerJSON), resolved, sessionExpiry(inner), nil
}

func textField(object map[string]any, key string) string {
	value, exists := object[key]
	if !exists || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func sessionExpiry(inner map[string]any) string {
	for _, key := range []string{"access_token", "refresh_token"} {
		if expiry := jwtExpiry(textField(inner, key)); !expiry.IsZero() {
			return expiry.Format(time.RFC3339)
		}
	}
	return ""
}

func jwtExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp <= 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0)
}
