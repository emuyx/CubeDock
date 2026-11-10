// Package redact masks credentials in diagnostic strings without hiding the
// surrounding error text.
package redact

import (
	"regexp"
	"strings"
)

const maxReportRunes = 500

var (
	embeddedJWT = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)
	longBlob    = regexp.MustCompile(`[A-Za-z0-9+/_-]{80,}={0,2}`)
	// Keys are matched with optional JSON escaping (\") so credentials inside a
	// doubly-encoded value such as {"sauth_json":"{\"access_token\":\"...\"}"}
	// are still redacted. Longer keys come before their suffixes.
	secretField = regexp.MustCompile(`(?i)(\\?"?(?:sauth_json|access_token|refresh_token|login_token|otp_token|client_secret|client_token|token|password|passwd|pwd|sessionid|cookie|chaininfo|sdkuid|udid|deviceid|userid|xuid|jwt)\\?"?\s*[:=]\s*)(\\?"(?:\\.|[^"\\])*\\?"|[^,;}\s]+)`)
)

type substitution struct {
	pattern     *regexp.Regexp
	replacement string
}

var substitutions = []substitution{
	{embeddedJWT, "[已隐藏JWT]"},
	{longBlob, "[已隐藏数据]"},
	{secretField, "$1[已隐藏]"},
}

// Sanitize removes the operator token and obvious credential values while
// preserving the actual error reason.
func Sanitize(text, token string) string {
	if token != "" {
		text = strings.ReplaceAll(text, token, "[已隐藏令牌]")
	}
	for _, rule := range substitutions {
		text = rule.pattern.ReplaceAllString(text, rule.replacement)
	}
	return clipRunes(text, maxReportRunes)
}

func clipRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}
