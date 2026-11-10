package core

import (
	"encoding/json"
	"strings"
)

// FormatTokens serialises room sessions for room-tokens.dat. JSON is used
// because a session may contain whitespace that newline-separated storage
// would split into multiple bogus tokens on the next start.
func FormatTokens(tokens []string) string {
	raw, err := json.Marshal(tokens)
	if err != nil {
		return ""
	}
	return string(raw)
}

// ParseTokens accepts the JSON array written by FormatTokens. Older data files
// stored raw sessions separated by whitespace, so that form is still accepted.
func ParseTokens(stored string) []string {
	trimmed := strings.TrimSpace(stored)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var tokens []string
		if json.Unmarshal([]byte(trimmed), &tokens) == nil {
			return tokens
		}
	}
	return strings.Fields(trimmed)
}
