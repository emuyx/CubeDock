package core

import (
	"reflect"
	"testing"
)

func TestTokensRoundTripPreservesWhitespace(t *testing.T) {
	tokens := []string{
		`{"sauth_json":"line one\nline two"}`,
		"b64:YWJj",
		"plain-token",
	}
	got := ParseTokens(FormatTokens(tokens))
	if !reflect.DeepEqual(got, tokens) {
		t.Fatalf("round trip = %#v, want %#v", got, tokens)
	}
}

func TestParseTokensAcceptsLegacyWhitespaceFormat(t *testing.T) {
	got := ParseTokens("token-a\ntoken-b  token-c")
	want := []string{"token-a", "token-b", "token-c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy parse = %#v, want %#v", got, want)
	}
}

func TestParseTokensHandlesEmptyInput(t *testing.T) {
	if got := ParseTokens("  \n"); got != nil {
		t.Fatalf("empty input = %#v", got)
	}
}
