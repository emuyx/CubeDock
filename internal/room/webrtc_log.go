package room

import (
	"context"
	"log/slog"

	"github.com/Happy2018new/nemc-tan-lobby-solver/core/nethernet"
)

// webrtcLogger forwards WebRTC stage names without SDP, candidates or TURN
// credentials.
type webrtcLogger struct{ trace func(string) }

func (w webrtcLogger) Enabled(context.Context, slog.Level) bool { return true }

func (w webrtcLogger) Handle(_ context.Context, record slog.Record) error {
	line := "WebRTC：" + record.Message
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key == "error" {
			line += "；" + attr.Value.String()
		}
		return true
	})
	if w.trace != nil {
		w.trace(line)
	}
	return nil
}

func (w webrtcLogger) WithAttrs([]slog.Attr) slog.Handler { return w }
func (w webrtcLogger) WithGroup(string) slog.Handler      { return w }

// signalingTrace records only signal types, never payloads.
type signalingTrace struct {
	nethernet.Signaling
	trace func(string)
}

func (s signalingTrace) Notify(inner nethernet.Notifier) func() {
	return s.Signaling.Notify(guestSignalTrace{Notifier: inner, trace: s.trace})
}

func (s signalingTrace) Signal(signal *nethernet.Signal) error {
	if s.trace != nil {
		s.trace("发送 WebRTC 信令：" + signal.Type)
	}
	return s.Signaling.Signal(signal)
}

type guestSignalTrace struct {
	nethernet.Notifier
	trace func(string)
}

func (g guestSignalTrace) NotifySignal(signal *nethernet.Signal) {
	if g.trace != nil {
		g.trace("收到 WebRTC 信令：" + signal.Type)
	}
	g.Notifier.NotifySignal(signal)
}
