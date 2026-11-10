package redact

import (
	"strings"
	"testing"
)

func TestSanitizeRedactsCredentialForms(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"unquoted access token", `auth failed: access_token=SECRET123`},
		{"quoted token", `{"token":"SECRET123"}`},
		{"refresh token", `{"refresh_token":"SECRET123"}`},
		{"otp token", `{"otp_token":"SECRET123"}`},
		{"password", `{"password":"SECRET123"}`},
		{"client secret", `{"client_secret":"SECRET123"}`},
		{"nested json", `{"sauth_json":"{\"access_token\":\"SECRET123\"}"}`},
		{"escaped json", `{\"token\":\"SECRET123\"}`},
		{"login token", `login_token: SECRET123`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Sanitize(tc.input, "")
			if strings.Contains(got, "SECRET123") {
				t.Fatalf("secret survived redaction: %q", got)
			}
		})
	}
}

func TestSanitizeKeepsNonCredentialFields(t *testing.T) {
	input := `{"token_set":false,"token_count":3}`
	if got := Sanitize(input, ""); got != input {
		t.Fatalf("non-credential fields changed: %q", got)
	}
}

func TestSanitizeReplacesExactToken(t *testing.T) {
	const token = "raw-session-value"
	got := Sanitize("upstream rejected "+token, token)
	if strings.Contains(got, token) {
		t.Fatalf("exact token survived: %q", got)
	}
}

func TestSanitizeClipsLongText(t *testing.T) {
	got := Sanitize(strings.Repeat("错", maxReportRunes+50), "")
	if len([]rune(got)) != maxReportRunes+1 {
		t.Fatalf("clipped length = %d", len([]rune(got)))
	}
}
