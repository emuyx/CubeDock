package room

import (
	"testing"
	"time"
)

func TestReadyQueueCancelsSupersededAndConnectedPlayers(t *testing.T) {
	done := make(chan struct{})
	defer close(done)
	queue := newReadyQueue(done, func() error { t.Error("cancelled retry sent a packet"); return nil }, nil)
	queue.delays = []time.Duration{time.Hour}

	queue.start(1, "first")
	queue.mu.Lock()
	first := queue.pending[1]
	queue.mu.Unlock()
	queue.start(1, "second")
	select {
	case <-first.stop:
	default:
		t.Fatal("duplicate join did not cancel the earlier retry")
	}
	queue.stopNetwork("second")
	queue.mu.Lock()
	pending := len(queue.pending)
	queue.mu.Unlock()
	if pending != 0 {
		t.Fatal("connected client retained retries")
	}

	queue.start(2, "third")
	queue.stopPlayer(2)
	queue.mu.Lock()
	pending = len(queue.pending)
	queue.mu.Unlock()
	if pending != 0 {
		t.Fatal("departed client retained retries")
	}
}

func TestReadyQueueFiniteAndStopsOnClose(t *testing.T) {
	done := make(chan struct{})
	sent := make(chan struct{}, 4)
	queue := newReadyQueue(done, func() error { sent <- struct{}{}; return nil }, nil)
	queue.delays = []time.Duration{0, 0}
	queue.start(1, "network")
	for range 2 {
		select {
		case <-sent:
		case <-time.After(time.Second):
			t.Fatal("retry not sent")
		}
	}
	deadline := time.After(time.Second)
	for {
		queue.mu.Lock()
		pending := len(queue.pending)
		queue.mu.Unlock()
		if pending == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("retry window did not finish")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	select {
	case <-sent:
		t.Fatal("unexpected extra retry")
	default:
	}

	queue.delays = []time.Duration{time.Hour}
	queue.start(2, "later")
	close(done)
	deadline = time.After(time.Second)
	for {
		queue.mu.Lock()
		pending := len(queue.pending)
		queue.mu.Unlock()
		if pending == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("room close leaked retries")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}
