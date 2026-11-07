package room

import (
	"sync"
	"time"
)

// retryDelays covers a guest that is still downloading components when its
// first "server ready" notification arrives.
var retryDelays = []time.Duration{
	2 * time.Second, 3 * time.Second, 5 * time.Second, 10 * time.Second,
	20 * time.Second, 20 * time.Second, 30 * time.Second, 30 * time.Second,
}

const maxRetryCampaigns = 64

type retryCampaign struct {
	network string
	cancel  chan struct{}
}

type readyQueue struct {
	mu        sync.Mutex
	campaigns map[uint32]*retryCampaign
	closed    <-chan struct{}
	send      func() error
	trace     func(string)
	schedule  []time.Duration
}

func newReadyQueue(closed <-chan struct{}, send func() error, trace func(string)) *readyQueue {
	return &readyQueue{
		campaigns: make(map[uint32]*retryCampaign),
		closed:    closed,
		send:      send,
		trace:     trace,
		schedule:  retryDelays,
	}
}

func (q *readyQueue) start(player uint32, network string) {
	q.mu.Lock()
	if previous := q.campaigns[player]; previous != nil {
		close(previous.cancel)
		delete(q.campaigns, player)
	}
	if len(q.campaigns) >= maxRetryCampaigns {
		q.mu.Unlock()
		return
	}
	campaign := &retryCampaign{network: network, cancel: make(chan struct{})}
	q.campaigns[player] = campaign
	q.mu.Unlock()

	go q.runCampaign(player, campaign)
}

func (q *readyQueue) runCampaign(player uint32, campaign *retryCampaign) {
	defer q.forget(player, campaign)
	for _, delay := range q.schedule {
		if !q.wait(campaign, delay) {
			return
		}
		if !q.stillWaiting(campaign) {
			return
		}
		if err := q.send(); err != nil {
			return
		}
		if q.trace != nil {
			q.trace("已补发服务器就绪通知，等待客户端下载组件并连接")
		}
	}
}

func (q *readyQueue) wait(campaign *retryCampaign, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-q.closed:
		return false
	case <-campaign.cancel:
		return false
	case <-timer.C:
		return true
	}
}

func (q *readyQueue) stillWaiting(campaign *retryCampaign) bool {
	select {
	case <-q.closed:
		return false
	case <-campaign.cancel:
		return false
	default:
		return true
	}
}

func (q *readyQueue) forget(player uint32, campaign *retryCampaign) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.campaigns[player] == campaign {
		delete(q.campaigns, player)
	}
}

func (q *readyQueue) stopPlayer(player uint32) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if campaign := q.campaigns[player]; campaign != nil {
		close(campaign.cancel)
		delete(q.campaigns, player)
	}
}

func (q *readyQueue) stopNetwork(network string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for player, campaign := range q.campaigns {
		if campaign.network == network {
			close(campaign.cancel)
			delete(q.campaigns, player)
		}
	}
}
