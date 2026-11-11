package netease

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseSession accepts the normalized 互通版 cookie form
// {"sauth_json":"<inner sauth JSON>"} and its double-encoded variants.
func ParseSession(input string) (Session, error) {
	var value any
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &value); err != nil {
		return Session{}, fmt.Errorf("会话 JSON 格式无效")
	}
	// Launcher cookies are commonly JSON-encoded more than once.
	for range 3 {
		text, ok := value.(string)
		if !ok {
			break
		}
		value = nil
		if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &value); err != nil {
			return Session{}, fmt.Errorf("会话 JSON 格式无效")
		}
	}
	outer, ok := value.(map[string]any)
	if !ok {
		return Session{}, fmt.Errorf("会话 JSON 必须是对象")
	}
	inner := outer
	if raw, exists := outer["sauth_json"]; exists {
		switch typed := raw.(type) {
		case string:
			var parsed map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSpace(typed)), &parsed); err != nil {
				return Session{}, fmt.Errorf("sauth_json 字段不是有效的 JSON 对象")
			}
			inner = parsed
		case map[string]any:
			inner = typed
		default:
			return Session{}, fmt.Errorf("sauth_json 字段类型无效")
		}
	}

	for _, field := range []string{"sdkuid", "sessionid", "udid", "deviceid"} {
		if textField(inner, field) == "" {
			return Session{}, fmt.Errorf("会话缺少字段 %s", field)
		}
	}
	session := Session{
		SDKUID:      textField(inner, "sdkuid"),
		SessionID:   textField(inner, "sessionid"),
		UDID:        textField(inner, "udid"),
		DeviceID:    textField(inner, "deviceid"),
		Platform:    strings.ToLower(textField(inner, "platform")),
		AppChannel:  textField(inner, "app_channel"),
		AccessToken: textField(inner, "access_token"),
		Raw:         inner,
	}
	if session.Platform != "pc" && session.AppChannel != "a50_sdk_cn" {
		label := session.Platform
		if label == "" {
			label = session.AppChannel
		}
		if label == "" {
			label = "未知"
		}
		return Session{}, fmt.Errorf("这是普通手游会话（%s），当前需要网易发烧平台启动的基岩互通版会话", label)
	}
	return session, nil
}

func textField(object map[string]any, key string) string {
	value, exists := object[key]
	if !exists || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
