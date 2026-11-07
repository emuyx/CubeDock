package relay

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/emuyx/cubedock/internal/relay/relaytest"
)

func TestPipePreservesPacketsAndMarker(t *testing.T) {
	client, server := relaytest.New(), relaytest.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Pipe(ctx, client, server) }()

	for _, payload := range [][]byte{{0, 1, 2}, {0xff, 0xfe, 0, 0x80}, bytes.Repeat([]byte{0x13}, 15000)} {
		client.In <- append([]byte(nil), payload...)
		select {
		case got := <-server.Out:
			if !bytes.Equal(got, append([]byte{0xfe}, payload...)) {
				t.Fatal("client packet changed on the way to RakNet")
			}
		case <-time.After(time.Second):
			t.Fatal("client forwarding stalled")
		}
		server.In <- append([]byte{0xfe}, payload...)
		select {
		case got := <-client.Out:
			if !bytes.Equal(got, payload) {
				t.Fatal("server packet changed on the way to NetherNet")
			}
		case <-time.After(time.Second):
			t.Fatal("server forwarding stalled")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancellation leaked forwarding goroutines")
	}
}

func TestPipeClosesBothPeersOnEOF(t *testing.T) {
	client, server := relaytest.New(), relaytest.New()
	done := make(chan error, 1)
	go func() { done <- Pipe(context.Background(), client, server) }()
	client.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("EOF did not stop the relay")
	}
	select {
	case <-server.Closed:
	default:
		t.Fatal("upstream left open")
	}
}
