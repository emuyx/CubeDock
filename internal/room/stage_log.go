package room

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"github.com/Happy2018new/nemc-tan-lobby-solver/core/nethernet"
	"github.com/pion/logging"
)

// stageLogger records WebRTC negotiation stages without SDP, candidates or TURN
// credentials.
type stageLogger struct{ trace func(string) }

func (l stageLogger) Enabled(context.Context, slog.Level) bool { return true }

func (l stageLogger) Handle(_ context.Context, record slog.Record) error {
	message := "WebRTC：" + record.Message
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "error" {
			message += "；" + attr.Value.String()
		}
		return true
	})
	if l.trace != nil {
		l.trace(message)
	}
	return nil
}

func (l stageLogger) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l stageLogger) WithGroup(string) slog.Handler      { return l }

// signalingObserver traces signal types, never their payloads.
type signalingObserver struct {
	nethernet.Signaling
	trace func(string)
}

func (o signalingObserver) Notify(notifier nethernet.Notifier) func() {
	return o.Signaling.Notify(notifierObserver{Notifier: notifier, trace: o.trace})
}

func (o signalingObserver) Signal(signal *nethernet.Signal) error {
	if o.trace != nil {
		o.trace("发送 WebRTC 信令：" + signal.Type)
	}
	return o.Signaling.Signal(signal)
}

type notifierObserver struct {
	nethernet.Notifier
	trace func(string)
}

func (n notifierObserver) NotifySignal(signal *nethernet.Signal) {
	if n.trace != nil {
		n.trace("收到 WebRTC 信令：" + signal.Type)
	}
	n.Notifier.NotifySignal(signal)
}

// transportLogger leaves most pion scopes silent. SCTP handshake metadata is
// useful when a relay stalls; cookies and payloads are always dropped.
type transportLogger struct{ trace func(string) }

func (f transportLogger) NewLogger(scope string) logging.LeveledLogger {
	if scope != "sctp" {
		return logging.NewDefaultLeveledLoggerForScope(scope, logging.LogLevelDisabled, io.Discard)
	}
	return logging.NewDefaultLeveledLoggerForScope(scope, logging.LogLevelDebug, transportWriter{trace: f.trace})
}

type transportWriter struct{ trace func(string) }

func (w transportWriter) Write(data []byte) (int, error) {
	line := strings.TrimSpace(string(data))
	if strings.Contains(strings.ToLower(line), "cookie") {
		return len(data), nil
	}
	for _, marker := range []string{
		"checksum", "unable to parse SCTP", "failed validating", "sending INIT",
		"chunkInit", "handshake", "readLoop exited",
	} {
		if strings.Contains(line, marker) {
			if w.trace != nil {
				w.trace(line)
			}
			break
		}
	}
	return len(data), nil
}
