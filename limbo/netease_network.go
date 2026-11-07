package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"

	nemcraknet "github.com/Happy2018new/nemc-tan-lobby-solver/core/raknet"
	"github.com/sandertv/gophertunnel/minecraft"
)

var (
	pullConnMu sync.Mutex
	pullConn   *nemcraknet.Conn
)

// takePullConn returns the most recent raw NetEase connection so the puller can
// send a RakNet DisconnectNotification before closing.
func takePullConn() *nemcraknet.Conn {
	pullConnMu.Lock()
	defer pullConnMu.Unlock()
	conn := pullConn
	pullConn = nil
	return conn
}

// neteaseV8 adapts CubeDock's vendored NetEase RakNet v8 client to the
// gophertunnel network interface. It is used only by the pack puller.
type neteaseV8 struct {
	logger *slog.Logger
}

func (n neteaseV8) DialContext(ctx context.Context, address string) (net.Conn, error) {
	conn, err := nemcraknet.DialContext(ctx, address)
	if err == nil {
		pullConnMu.Lock()
		pullConn = conn
		pullConnMu.Unlock()
	}
	return conn, err
}

func (n neteaseV8) PingContext(context.Context, string) ([]byte, error) {
	// NetEase v8 has no compatible RakNet ping in the vendored client; the
	// dialer falls back to DialContext when PingContext fails.
	return nil, errors.New("netease v8 ping is not supported")
}

func (n neteaseV8) Listen(string) (minecraft.NetworkListener, error) {
	return nil, errors.New("netease v8 listener is not supported")
}

func init() {
	minecraft.RegisterNetwork("netease-v8", func(logger *slog.Logger) minecraft.Network {
		return neteaseV8{logger: logger}
	})
}
