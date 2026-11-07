package room

import (
	"testing"
	"time"
)

func campaignCount(queue *readyQueue) int {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return len(queue.campaigns)
}

func waitForCampaigns(t *testing.T, queue *readyQueue, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if campaignCount(queue) == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("campaigns = %d, want %d", campaignCount(queue), want)
}

func TestReadyQueueCancelsSupersededCampaign(t *testing.T) {
	closed := make(chan struct{})
	defer close(closed)
	queue := newReadyQueue(closed, func() error {
		t.Error("cancelled campaign sent a notification")
		return nil
	}, nil)
	queue.schedule = []time.Duration{time.Hour}

	queue.start(1, "first")
	queue.mu.Lock()
	first := queue.campaigns[1]
	queue.mu.Unlock()

	queue.start(1, "second")
	select {
	case <-first.cancel:
	default:
		t.Fatal("duplicate join did not cancel the first campaign")
	}
	queue.stopNetwork("second")
	waitForCampaigns(t, queue, 0)

	queue.start(2, "third")
	queue.stopPlayer(2)
	waitForCampaigns(t, queue, 0)
}

func TestReadyQueueFiniteScheduleAndShutdown(t *testing.T) {
	closed := make(chan struct{})
	sent := make(chan struct{}, 4)
	queue := newReadyQueue(closed, func() error {
		sent <- struct{}{}
		return nil
	}, nil)
	queue.schedule = []time.Duration{0, 0}
	queue.start(1, "network")

	for attempt := 0; attempt < 2; attempt++ {
		select {
		case <-sent:
		case <-time.After(time.Second):
			t.Fatal("retry notification was not sent")
		}
	}
	waitForCampaigns(t, queue, 0)
	select {
	case <-sent:
		t.Fatal("schedule produced more notifications than allowed")
	default:
	}

	queue.schedule = []time.Duration{time.Hour}
	queue.start(2, "later")
	close(closed)
	waitForCampaigns(t, queue, 0)
}
