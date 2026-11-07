// Package relaytest provides an in-memory relay.Stream for tests.
package relaytest

import (
	"errors"
	"io"
	"sync"
	"time"
)

// ErrTimeout is returned by Next when no packet arrives in time.
var ErrTimeout = errors.New("relaytest: receive timeout")

// Packets is a connected pair of packet queues implementing relay.Stream.
type Packets struct {
	inbound  chan []byte
	outbound chan []byte
	done     chan struct{}
	once     sync.Once
}

func New() *Packets {
	return &Packets{
		inbound:  make(chan []byte, 4),
		outbound: make(chan []byte, 4),
		done:     make(chan struct{}),
	}
}

// Send queues a packet for the peer's ReadPacket.
func (p *Packets) Send(packet []byte) bool {
	return p.enqueue(p.inbound, packet)
}

// Next waits for a packet written by the peer.
func (p *Packets) Next(timeout time.Duration) ([]byte, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case packet := <-p.outbound:
		return packet, nil
	case <-p.done:
		return nil, io.EOF
	case <-timer.C:
		return nil, ErrTimeout
	}
}

// Done is closed when the stream is shut down.
func (p *Packets) Done() <-chan struct{} {
	return p.done
}

func (p *Packets) ReadPacket() ([]byte, error) {
	select {
	case <-p.done:
		return nil, io.EOF
	case packet := <-p.inbound:
		return packet, nil
	}
}

func (p *Packets) Write(packet []byte) (int, error) {
	if !p.enqueue(p.outbound, packet) {
		return 0, io.EOF
	}
	return len(packet), nil
}

func (p *Packets) enqueue(queue chan<- []byte, packet []byte) bool {
	copied := append([]byte(nil), packet...)
	select {
	case <-p.done:
		return false
	case queue <- copied:
		return true
	}
}

func (p *Packets) Close() error {
	p.once.Do(func() { close(p.done) })
	return nil
}
