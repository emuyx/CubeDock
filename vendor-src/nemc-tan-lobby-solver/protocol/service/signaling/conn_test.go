package signaling

import (
	"strings"
	"testing"
)

func TestConnLogfIsOptional(t *testing.T) {
	(&Conn{}).logf("no logger configured")
}

func TestConnLogfReceivesCredentialFreeLine(t *testing.T) {
	var lines []string
	conn := &Conn{dialer: Dialer{Logf: func(line string) { lines = append(lines, line) }}}
	conn.logf("收到信令消息：from=%s type=%d", "123", 2)
	if len(lines) != 1 {
		t.Fatalf("log lines = %v", lines)
	}
	if !strings.Contains(lines[0], "from=123") || strings.Contains(lines[0], "signal:") {
		t.Fatalf("unexpected log line: %q", lines[0])
	}
}
