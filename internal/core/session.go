package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// NormalizeInteropSession accepts a 基岩互通版 session copied out of the Fever
// Games launcher in any of the shapes the console may receive: the inner sauth
// object, a {"sauth_json":"..."} cookie, or a JSON string containing either.
// It returns the token to store, the storage platform label and the
// access_token expiry when the token is a JWT.
func NormalizeInteropSession(input string) (token, platform, expiresAt string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", "", fmt.Errorf("请粘贴互通版会话 sauth_json")
	}
	var value any
	if json.Unmarshal([]byte(input), &value) != nil {
		return "", "", "", fmt.Errorf("会话 JSON 格式无效")
	}
	// Pasted launcher cookies are commonly JSON-encoded twice because
	// sauth_json is itself a string.
	for range 3 {
		text, ok := value.(string)
		if !ok {
			break
		}
		value = nil
		if json.Unmarshal([]byte(strings.TrimSpace(text)), &value) != nil {
			return "", "", "", fmt.Errorf("会话 JSON 格式无效")
		}
	}
	outer, ok := value.(map[string]any)
	if !ok {
		return "", "", "", fmt.Errorf("会话 JSON 必须是对象")
	}
	inner := outer
	if raw, exists := outer["sauth_json"]; exists {
		switch typed := raw.(type) {
		case string:
			var parsed map[string]any
			if json.Unmarshal([]byte(strings.TrimSpace(typed)), &parsed) != nil {
				return "", "", "", fmt.Errorf("sauth_json 字段不是有效的 JSON 对象")
			}
			inner = parsed
		case map[string]any:
			inner = typed
		default:
			return "", "", "", fmt.Errorf("sauth_json 字段类型无效")
		}
	}

	for _, field := range []string{"sdkuid", "sessionid", "udid", "deviceid"} {
		if textField(inner, field) == "" {
			return "", "", "", fmt.Errorf("会话缺少字段 %s", field)
		}
	}
	reportedPlatform := strings.ToLower(textField(inner, "platform"))
	channel := textField(inner, "app_channel")
	if reportedPlatform != "pc" && channel != "a50_sdk_cn" {
		label := reportedPlatform
		if label == "" || label == "<nil>" {
			label = channel
		}
		if label == "" || label == "<nil>" {
			label = "ad"
		}
		return "", "", "", fmt.Errorf("这是普通手游会话（%s），当前网易只接受网易发烧平台启动的基岩互通版会话", label)
	}
	if channel == "" {
		inner["app_channel"] = "a50_sdk_cn"
	}
	if reportedPlatform == "" {
		inner["platform"] = "pc"
	}

	compact, err := json.Marshal(inner)
	if err != nil {
		return "", "", "", fmt.Errorf("序列化会话失败：%w", err)
	}
	outerJSON, err := json.Marshal(map[string]string{"sauth_json": string(compact)})
	if err != nil {
		return "", "", "", fmt.Errorf("序列化 Cookie 失败：%w", err)
	}
	// The protocol reports platform=pc, but this is the mobile engine running
	// through the Windows launcher, not the Bedrock PC edition.
	return string(outerJSON), "interop", sessionExpiry(inner), nil
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
