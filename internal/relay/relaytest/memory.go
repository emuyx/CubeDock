// Package relaytest offers an in-memory relay.Stream for tests.
package relaytest

import (
	"io"
	"sync"
)

// Packets is a relay.Stream backed by channels.
type Packets struct {
	In     chan []byte
	Out    chan []byte
	Closed chan struct{}
	once   sync.Once
}

// New returns a connected in-memory stream.
func New() *Packets {
	return &Packets{
		In:     make(chan []byte, 4),
		Out:    make(chan []byte, 4),
		Closed: make(chan struct{}),
	}
}

func (p *Packets) ReadPacket() ([]byte, error) {
	select {
	case <-p.Closed:
		return nil, io.EOF
	case packet := <-p.In:
		return packet, nil
	}
}

func (p *Packets) Write(packet []byte) (int, error) {
	select {
	case <-p.Closed:
		return 0, io.EOF
	case p.Out <- append([]byte(nil), packet...):
		return len(packet), nil
	}
}

func (p *Packets) Close() error {
	p.once.Do(func() { close(p.Closed) })
	return nil
}
