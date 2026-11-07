package relay

import "sync"

// SyncStream serializes writes to a Stream. Relay forwarding and operator
// actions such as injecting a Transfer packet may write concurrently; the
// underlying NetherNet data channel expects complete messages.
type SyncStream struct {
	Stream
	mu sync.Mutex
}

func (s *SyncStream) Write(packet []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Stream.Write(packet)
}
