package redact

import (
	"regexp"
	"strings"
)

var (
	jwtPattern = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)
	longData   = regexp.MustCompile(`[A-Za-z0-9+/_-]{80,}={0,2}`)
	fieldValue = regexp.MustCompile(`(?i)("?(?:sauth_json|access_token|login_token|sessionid|cookie|chaininfo|sdkuid|udid|deviceid|userid|xuid)"?\s*[:=]\s*)("[^"]*"|[^,;}\s]+)`)
)

// Sanitize removes the operator's token and obvious credential values while
// keeping the actual error reason visible. Field names such as sauth_json or
// cookie are not secrets by themselves and must not hide unrelated errors.
func Sanitize(s, token string) string {
	if token != "" {
		s = strings.ReplaceAll(s, token, "[已隐藏令牌]")
	}
	s = jwtPattern.ReplaceAllString(s, "[已隐藏JWT]")
	s = longData.ReplaceAllString(s, "[已隐藏数据]")
	s = fieldValue.ReplaceAllString(s, "$1[已隐藏]")
	if len([]rune(s)) > 500 {
		s = string([]rune(s)[:500]) + "…"
	}
	return s
}
