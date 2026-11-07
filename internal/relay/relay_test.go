package relay

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/emuyx/cubedock/internal/relay/relaytest"
)

type pipeFixture struct {
	client *relaytest.Packets
	server *relaytest.Packets
	cancel context.CancelFunc
	done   chan error
}

func newPipeFixture(t *testing.T) *pipeFixture {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	fixture := &pipeFixture{
		client: relaytest.New(),
		server: relaytest.New(),
		cancel: cancel,
		done:   make(chan error, 1),
	}
	go func() { fixture.done <- Pipe(ctx, fixture.client, fixture.server) }()
	return fixture
}

func (f *pipeFixture) stop(t *testing.T) {
	t.Helper()
	f.cancel()
	select {
	case <-f.done:
	case <-time.After(time.Second):
		t.Fatal("relay goroutines survived cancellation")
	}
}

func TestRelayAddsAndRemovesMarker(t *testing.T) {
	fixture := newPipeFixture(t)
	defer fixture.stop(t)

	payloads := [][]byte{
		{0, 1, 2},
		{0xff, 0xfe, 0, 0x80},
		bytes.Repeat([]byte{0x13}, 15000),
	}
	for _, payload := range payloads {
		if !fixture.client.Send(payload) {
			t.Fatal("client stream closed unexpectedly")
		}
		upstream, err := fixture.server.Next(time.Second)
		if err != nil {
			t.Fatalf("upstream receive: %v", err)
		}
		if !bytes.Equal(upstream, append([]byte{0xfe}, payload...)) {
			t.Fatal("NetherNet payload changed while adding the RakNet marker")
		}

		if !fixture.server.Send(append([]byte{0xfe}, payload...)) {
			t.Fatal("server stream closed unexpectedly")
		}
		downstream, err := fixture.client.Next(time.Second)
		if err != nil {
			t.Fatalf("client receive: %v", err)
		}
		if !bytes.Equal(downstream, payload) {
			t.Fatal("RakNet payload changed while removing the marker")
		}
	}
}

func TestClientCloseClosesUpstream(t *testing.T) {
	fixture := newPipeFixture(t)
	defer fixture.cancel()

	_ = fixture.client.Close()
	select {
	case <-fixture.done:
	case <-time.After(time.Second):
		t.Fatal("client EOF did not stop the relay")
	}
	select {
	case <-fixture.server.Done():
	default:
		t.Fatal("upstream was left open after client EOF")
	}
}
