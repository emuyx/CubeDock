package room

import (
	"sync"
	"time"
)

// retrySchedule covers a client still downloading components when its first
// "server ready" notification arrives. Each attempt is cancelled as soon as the
// guest connects, leaves, or the room closes.
var retrySchedule = []time.Duration{
	2 * time.Second, 3 * time.Second, 5 * time.Second, 10 * time.Second,
	20 * time.Second, 20 * time.Second, 30 * time.Second, 30 * time.Second,
}

type readyAttempt struct {
	network string
	stop    chan struct{}
}

type readyQueue struct {
	mu      sync.Mutex
	pending map[uint32]*readyAttempt
	done    <-chan struct{}
	notify  func() error
	trace   func(string)
	delays  []time.Duration
}

func newReadyQueue(done <-chan struct{}, notify func() error, trace func(string)) *readyQueue {
	return &readyQueue{
		pending: make(map[uint32]*readyAttempt),
		done:    done,
		notify:  notify,
		trace:   trace,
		delays:  retrySchedule,
	}
}

func (q *readyQueue) start(player uint32, network string) {
	q.mu.Lock()
	if previous := q.pending[player]; previous != nil {
		close(previous.stop)
		delete(q.pending, player)
	}
	if len(q.pending) >= 64 {
		q.mu.Unlock()
		return
	}
	attempt := &readyAttempt{network: network, stop: make(chan struct{})}
	q.pending[player] = attempt
	q.mu.Unlock()

	go func() {
		defer func() {
			q.mu.Lock()
			if q.pending[player] == attempt {
				delete(q.pending, player)
			}
			q.mu.Unlock()
		}()
		for _, delay := range q.delays {
			timer := time.NewTimer(delay)
			select {
			case <-q.done:
				timer.Stop()
				return
			case <-attempt.stop:
				timer.Stop()
				return
			case <-timer.C:
			}
			// A connection may have arrived at the same instant as the timer.
			select {
			case <-q.done:
				return
			case <-attempt.stop:
				return
			default:
			}
			if q.notify() != nil {
				return
			}
			if q.trace != nil {
				q.trace("已补发服务器就绪通知，等待客户端下载组件并连接")
			}
		}
	}()
}

func (q *readyQueue) stopPlayer(player uint32) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if attempt := q.pending[player]; attempt != nil {
		close(attempt.stop)
		delete(q.pending, player)
	}
}

func (q *readyQueue) stopNetwork(network string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for player, attempt := range q.pending {
		if attempt.network == network {
			close(attempt.stop)
			delete(q.pending, player)
		}
	}
}
