package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emuyx/cubedock/internal/relay"
)

// PlayerState is the lifecycle stage of one relayed guest connection.
type PlayerState string

const (
	PlayerConnecting     PlayerState = "connecting"
	PlayerAuthenticating PlayerState = "authenticating"
	PlayerJoined         PlayerState = "joined"
	PlayerLoginFailed    PlayerState = "login_failed"
	PlayerKicked         PlayerState = "kicked"
)

// PlayerStatus is the console-visible snapshot of one live guest. NetEase UIDs
// are intentionally not exposed.
type PlayerStatus struct {
	ID            string      `json:"id"`
	Name          string      `json:"name,omitempty"`
	Room          int         `json:"room"`
	RoomID        uint32      `json:"room_id,omitempty"`
	Account       string      `json:"account,omitempty"`
	State         PlayerState `json:"state"`
	StartedAt     string      `json:"started_at"`
	JoinedAt      string      `json:"joined_at,omitempty"`
	UptimeSeconds int64       `json:"uptime_seconds"`
	UpPackets     uint64      `json:"up_packets"`
	DownPackets   uint64      `json:"down_packets"`
}

type livePlayer struct {
	id        string
	roomIndex int
	roomID    uint32
	account   string
	startedAt time.Time
	cancel    context.CancelFunc

	name     string
	state    PlayerState
	joinedAt time.Time

	up     *atomic.Uint64
	down   *atomic.Uint64
	client *relay.SyncStream
}

type playerRegistry struct {
	mu      sync.RWMutex
	nextID  uint64
	players map[string]*livePlayer
}

func (r *playerRegistry) register(roomIndex int, roomID uint32, account string, cancel context.CancelFunc) *livePlayer {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.players == nil {
		r.players = make(map[string]*livePlayer)
	}
	r.nextID++
	player := &livePlayer{
		id:        fmt.Sprintf("p%d", r.nextID),
		roomIndex: roomIndex,
		roomID:    roomID,
		account:   account,
		startedAt: time.Now(),
		cancel:    cancel,
		state:     PlayerConnecting,
	}
	r.players[player.id] = player
	return player
}

func (r *playerRegistry) unregister(player *livePlayer) {
	if player == nil {
		return
	}
	r.mu.Lock()
	if current, ok := r.players[player.id]; ok && current == player {
		delete(r.players, player.id)
	}
	r.mu.Unlock()
}

func (r *playerRegistry) attachStreams(player *livePlayer, up, down *atomic.Uint64) {
	r.mu.Lock()
	if current, ok := r.players[player.id]; ok && current == player {
		player.up, player.down = up, down
	}
	r.mu.Unlock()
}

func (r *playerRegistry) attachClient(player *livePlayer, client *relay.SyncStream) {
	r.mu.Lock()
	if current, ok := r.players[player.id]; ok && current == player {
		player.client = client
	}
	r.mu.Unlock()
}

func (r *playerRegistry) noteLogin(player *livePlayer, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.currentLocked(player) || player.state == PlayerKicked {
		return
	}
	if name != "" {
		player.name = name
	}
	if player.state == PlayerConnecting {
		player.state = PlayerAuthenticating
	}
}

func (r *playerRegistry) notePlayStatus(player *livePlayer, status int32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.currentLocked(player) || player.state == PlayerKicked {
		return
	}
	if status == 0 {
		player.state = PlayerJoined
		if player.joinedAt.IsZero() {
			player.joinedAt = time.Now()
		}
		return
	}
	player.state = PlayerLoginFailed
}

func (r *playerRegistry) currentLocked(player *livePlayer) bool {
	current, ok := r.players[player.id]
	return ok && current == player
}

func (r *playerRegistry) list() []PlayerStatus {
	now := time.Now()

	r.mu.RLock()
	entries := make([]*livePlayer, 0, len(r.players))
	for _, player := range r.players {
		entries = append(entries, player)
	}
	out := make([]PlayerStatus, 0, len(entries))
	for _, player := range entries {
		out = append(out, playerStatus(player, now))
	}
	r.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].Room != out[j].Room {
			return out[i].Room < out[j].Room
		}
		if out[i].StartedAt != out[j].StartedAt {
			return out[i].StartedAt < out[j].StartedAt
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func playerStatus(player *livePlayer, now time.Time) PlayerStatus {
	status := PlayerStatus{
		ID:        player.id,
		Name:      player.name,
		Room:      player.roomIndex + 1,
		RoomID:    player.roomID,
		Account:   player.account,
		State:     player.state,
		StartedAt: player.startedAt.Format(time.RFC3339),
	}
	if !player.joinedAt.IsZero() {
		status.JoinedAt = player.joinedAt.Format(time.RFC3339)
	}
	if uptime := now.Sub(player.startedAt); uptime > 0 {
		status.UptimeSeconds = int64(uptime.Seconds())
	}
	if player.up != nil {
		status.UpPackets = player.up.Load()
	}
	if player.down != nil {
		status.DownPackets = player.down.Load()
	}
	return status
}

func (r *playerRegistry) resolveLocked(target string) (*livePlayer, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("请指定玩家 ID 或名称")
	}
	if isPlayerID(target) {
		player := r.players[strings.ToLower(target)]
		if player == nil {
			return nil, fmt.Errorf("未找到玩家 %s", target)
		}
		return player, nil
	}
	var matches []*livePlayer
	for _, candidate := range r.players {
		if candidate.name != "" && strings.EqualFold(candidate.name, target) {
			matches = append(matches, candidate)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("未找到玩家 %s", target)
	case 1:
		return matches[0], nil
	default:
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].roomIndex != matches[j].roomIndex {
				return matches[i].roomIndex < matches[j].roomIndex
			}
			return matches[i].id < matches[j].id
		})
		candidates := make([]string, 0, len(matches))
		for _, match := range matches {
			candidates = append(candidates, fmt.Sprintf("%s@房间 %d", match.id, match.roomIndex+1))
		}
		return nil, fmt.Errorf("多个玩家匹配 %q，请使用玩家 ID：%s",
			target, strings.Join(candidates, "、"))
	}
}

// kick resolves target by canonical ID or exact case-insensitive name. The
// returned cancel func must be invoked after the registry lock is released.
func (r *playerRegistry) kick(target string) (PlayerStatus, context.CancelFunc, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	player, err := r.resolveLocked(target)
	if err != nil {
		return PlayerStatus{}, nil, err
	}
	if player.state == PlayerKicked {
		return PlayerStatus{}, nil, fmt.Errorf("玩家 %s 正在断开", player.id)
	}
	player.state = PlayerKicked
	return playerStatus(player, time.Now()), player.cancel, nil
}

// transferTarget resolves a joined player and returns the client stream. The
// caller writes the Transfer batch after the registry lock is released.
func (r *playerRegistry) transferTarget(target string) (PlayerStatus, *relay.SyncStream, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	player, err := r.resolveLocked(target)
	if err != nil {
		return PlayerStatus{}, nil, err
	}
	if player.state != PlayerJoined {
		return PlayerStatus{}, nil, fmt.Errorf("玩家 %s 尚未完成登录（%s）",
			player.id, playerStateLabel(player.state))
	}
	if player.client == nil {
		return PlayerStatus{}, nil, fmt.Errorf("玩家 %s 的客户端连接不可用", player.id)
	}
	return playerStatus(player, time.Now()), player.client, nil
}

func isPlayerID(target string) bool {
	lower := strings.ToLower(target)
	if len(lower) < 2 || lower[0] != 'p' {
		return false
	}
	for _, r := range lower[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ListPlayers returns a snapshot of guests relayed by every room.
func (m *Manager) ListPlayers() []PlayerStatus {
	return m.players.list()
}

// KickPlayer disconnects one guest by ID or exact name. The actual teardown is
// asynchronous; a second kick sees the intermediate "kicked" state.
func (m *Manager) KickPlayer(target string) (PlayerStatus, error) {
	status, cancel, err := m.players.kick(target)
	if err != nil {
		return PlayerStatus{}, err
	}
	if cancel != nil {
		cancel()
	}
	m.emit("控制台", fmt.Sprintf("管理员已断开玩家 %s（%s，房间 %d）",
		playerLabel(status.Name), status.ID, status.Room))
	return status, nil
}
