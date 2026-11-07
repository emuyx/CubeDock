// Package relay moves complete game packets between NetherNet and RakNet.
// NetherNet carries packets without the RakNet 0xfe datagram marker; only that
// marker is added or removed. Login and encrypted payload bytes pass untouched.
package relay

import (
	"context"
	"fmt"
	"io"
)

// Stream is the packet-level view of a connection.
type Stream interface {
	ReadPacket() ([]byte, error)
	Write([]byte) (int, error)
	Close() error
}

// Pipe forwards packets in both directions until either side fails or ctx is
// cancelled. Both streams are always closed before Pipe returns, and no relay
// goroutine outlives it.
func Pipe(ctx context.Context, client, server Stream) error {
	results := make(chan pipeResult, 2)
	go forward("client", client, server, true, results)
	go forward("server", server, client, false, results)

	cancelled := false
	var err error
	select {
	case result := <-results:
		err = fmt.Errorf("%s: %w", result.side, result.err)
	case <-ctx.Done():
		err = ctx.Err()
		cancelled = true
	}
	_ = client.Close()
	_ = server.Close()

	// Both reads unblock on Close; wait for every forwarding goroutine so a
	// stopped room cannot leave a live path behind.
	if cancelled {
		<-results
		<-results
	} else {
		<-results
	}
	return err
}

type pipeResult struct {
	side string
	err  error
}

func forward(side string, from, to Stream, toRakNet bool, results chan<- pipeResult) {
	for {
		packet, err := from.ReadPacket()
		if err != nil {
			results <- pipeResult{side: side, err: err}
			return
		}
		if toRakNet {
			packet = append([]byte{0xfe}, packet...)
		} else {
			if len(packet) == 0 || packet[0] != 0xfe {
				continue
			}
			packet = packet[1:]
		}
		written, err := to.Write(packet)
		if err == nil && written != len(packet) {
			err = io.ErrShortWrite
		}
		if err != nil {
			results <- pipeResult{side: side, err: err}
			return
		}
	}
}
