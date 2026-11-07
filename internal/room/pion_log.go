package room

import (
	"io"
	"strings"

	"github.com/pion/logging"
)

// pionLogger keeps non-SCTP pion scopes silent and forwards SCTP handshake
// markers through pionTraceWriter.
type pionLogger struct{ trace func(string) }

func (p pionLogger) NewLogger(scope string) logging.LeveledLogger {
	if scope != "sctp" {
		return logging.NewDefaultLeveledLoggerForScope(scope, logging.LogLevelDisabled, io.Discard)
	}
	return logging.NewDefaultLeveledLoggerForScope(scope, logging.LogLevelDebug, pionTraceWriter{trace: p.trace})
}

type pionTraceWriter struct{ trace func(string) }

func (w pionTraceWriter) Write(data []byte) (int, error) {
	line := strings.TrimSpace(string(data))
	if strings.Contains(strings.ToLower(line), "cookie") {
		return len(data), nil
	}
	if w.trace != nil && interestingSCTPLine(line) {
		w.trace(line)
	}
	return len(data), nil
}

func interestingSCTPLine(line string) bool {
	for _, marker := range []string{
		"checksum", "unable to parse SCTP", "failed validating", "sending INIT",
		"chunkInit", "handshake", "readLoop exited",
	} {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return false
}
