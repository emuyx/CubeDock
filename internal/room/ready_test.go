package room

import (
	"testing"
	"time"
)

func pendingCount(queue *readyQueue) int {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return len(queue.pending)
}

func waitForPending(t *testing.T, queue *readyQueue, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if pendingCount(queue) == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("pending retries = %d, want %d", pendingCount(queue), want)
}

func TestReadyQueueDropsSupersededAndClosedAttempts(t *testing.T) {
	done := make(chan struct{})
	defer close(done)
	queue := newReadyQueue(done, func() error {
		t.Error("a cancelled attempt sent a notification")
		return nil
	}, nil)
	queue.delays = []time.Duration{time.Hour}

	queue.start(1, "first")
	queue.mu.Lock()
	first := queue.pending[1]
	queue.mu.Unlock()

	queue.start(1, "second")
	select {
	case <-first.stop:
	default:
		t.Fatal("a second join did not stop the first retry window")
	}
	queue.stopNetwork("second")
	waitForPending(t, queue, 0)

	queue.start(2, "third")
	queue.stopPlayer(2)
	waitForPending(t, queue, 0)
}

func TestReadyQueueFiniteScheduleAndShutdown(t *testing.T) {
	done := make(chan struct{})
	sent := make(chan struct{}, 4)
	queue := newReadyQueue(done, func() error {
		sent <- struct{}{}
		return nil
	}, nil)
	queue.delays = []time.Duration{0, 0}
	queue.start(1, "network")

	for attempt := 0; attempt < 2; attempt++ {
		select {
		case <-sent:
		case <-time.After(time.Second):
			t.Fatal("retry notification was not sent")
		}
	}
	waitForPending(t, queue, 0)
	select {
	case <-sent:
		t.Fatal("more retries were sent than the schedule allows")
	default:
	}

	queue.delays = []time.Duration{time.Hour}
	queue.start(2, "later")
	close(done)
	waitForPending(t, queue, 0)
}
