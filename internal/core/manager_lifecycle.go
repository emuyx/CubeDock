// Package core owns CubeDock's room lifecycle, account list and event log. It
// has no HTTP dependencies so the console is only one possible front end.
package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/session"
)

// ErrRoomRunning means an account already has a room runtime.
var ErrRoomRunning = errors.New("该房间已经在运行")

type roomState uint8

const (
	roomStarting roomState = iota
	roomRunning
	roomStopping
)

// roomRuntime is one account's independently managed room.
type roomRuntime struct {
	index      int
	token      string
	profile    config.Profile
	cancel     context.CancelFunc
	done       chan struct{}
	lease      *session.Lease
	limbo      *limboLease
	state      roomState
	finishOnce sync.Once
}

func (m *Manager) actionLock(index int) *sync.Mutex {
	m.actionsMu.Lock()
	defer m.actionsMu.Unlock()
	lock := m.actions[index]
	if lock == nil {
		lock = &sync.Mutex{}
		m.actions[index] = lock
	}
	return lock
}

func (m *Manager) prepLock(profileID string) *sync.Mutex {
	m.prepMu.Lock()
	defer m.prepMu.Unlock()
	lock := m.prepLocks[profileID]
	if lock == nil {
		lock = &sync.Mutex{}
		m.prepLocks[profileID] = lock
	}
	return lock
}

// resolveAccountLocked returns the profile and token for one account.
func (m *Manager) resolveAccountLocked(index int) (config.Profile, string, error) {
	if index < 0 || index >= len(m.tokens) {
		return config.Profile{}, "", fmt.Errorf("账号序号无效")
	}
	record := m.accounts[index]
	profileID := record.ProfileID
	if profileID == "" {
		profileID = m.cfg.DefaultProfileID()
	}
	profile, ok := m.cfg.Profile(profileID)
	if !ok {
		return config.Profile{}, "", fmt.Errorf("房间 %d 选择的配置方案 %q 不存在", index+1, profileID)
	}
	return profile, m.tokens[index], nil
}

// StartAccount starts one account's room without touching any other room.
func (m *Manager) StartAccount(index int) error {
	lock := m.actionLock(index)
	lock.Lock()
	defer lock.Unlock()
	return m.startAccount(index)
}

func (m *Manager) startAccount(index int) error {
	m.mu.Lock()
	profile, token, err := m.resolveAccountLocked(index)
	m.mu.Unlock()
	if err != nil {
		return err
	}
	lease, err := m.claimSession(token)
	if err != nil {
		if len(m.tokens) > 1 {
			return fmt.Errorf("%s%w", roomLabel(index, len(m.tokens)), err)
		}
		return err
	}
	return m.startWithLease(index, profile, token, lease)
}

// Start starts every account that is not already running. The call is
// all-or-nothing for the rooms it creates: if any starts fails, those rooms are
// stopped again.
func (m *Manager) Start() error {
	m.mu.Lock()
	if len(m.tokens) == 0 {
		m.mu.Unlock()
		return fmt.Errorf("请先添加网易账号")
	}
	indexes := make([]int, 0, len(m.tokens))
	for index := range m.tokens {
		if _, running := m.runtimes[index]; !running {
			indexes = append(indexes, index)
		}
	}
	m.mu.Unlock()
	if len(indexes) == 0 {
		return nil
	}

	type pending struct {
		index   int
		profile config.Profile
		token   string
		lease   *session.Lease
	}
	claims := make([]pending, 0, len(indexes))
	m.lifecycleMu.Lock()
	releaseClaims := func() {
		for _, claim := range claims {
			claim.lease.Release()
		}
	}
	for _, index := range indexes {
		m.mu.Lock()
		profile, token, err := m.resolveAccountLocked(index)
		m.mu.Unlock()
		if err != nil {
			releaseClaims()
			m.lifecycleMu.Unlock()
			return err
		}
		lease, err := m.claimSession(token)
		if err != nil {
			releaseClaims()
			m.lifecycleMu.Unlock()
			if len(m.tokens) > 1 {
				return fmt.Errorf("%s%w", roomLabel(index, len(m.tokens)), err)
			}
			return err
		}
		claims = append(claims, pending{index: index, profile: profile, token: token, lease: lease})
	}
	m.lifecycleMu.Unlock()

	started := make([]int, 0, len(claims))
	consumed := make([]bool, len(claims))
	var firstErr error
	for position, claim := range claims {
		err := m.startWithLease(claim.index, claim.profile, claim.token, claim.lease)
		consumed[position] = true
		if err != nil {
			firstErr = err
			break
		}
		started = append(started, claim.index)
	}
	if firstErr == nil {
		return nil
	}
	for position, claim := range claims {
		if !consumed[position] {
			claim.lease.Release()
		}
	}
	for _, index := range started {
		_ = m.StopAccount(index)
	}
	for _, index := range started {
		m.waitRuntime(index, 15*time.Second)
	}
	return firstErr
}

// startWithLease registers a runtime and launches it. It takes ownership of
// lease and releases it in every outcome.
func (m *Manager) startWithLease(index int, profile config.Profile, token string, lease *session.Lease) error {
	m.mu.Lock()
	if _, exists := m.runtimes[index]; exists {
		m.mu.Unlock()
		lease.Release()
		return ErrRoomRunning
	}
	if len(m.runtimes) == 0 {
		m.idle = make(chan struct{})
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtime := &roomRuntime{
		index: index, token: token, profile: profile,
		cancel: cancel, done: make(chan struct{}), lease: lease,
		state: roomStarting,
	}
	m.runtimes[index] = runtime
	m.syncRoomsLocked()
	status := &m.status.Rooms[index]
	*status = RoomStatus{
		Phase:          "starting",
		Account:        firstNonEmpty(m.accounts[index].Name, m.accounts[index].Label, m.accounts[index].Email),
		ProfileID:      profile.ID,
		ProfileName:    profile.Name,
		ConnectionMode: profile.ConnectionMode,
	}
	m.mu.Unlock()

	abort := func(err error) error {
		m.finishRuntime(runtime)
		return err
	}

	if profile.ConnectionMode == config.ConnectionTransfer {
		if err := m.prepareProfile(ctx, profile); err != nil {
			return abort(err)
		}
		limboLease, err := m.acquireLimbo(profile)
		if err != nil {
			return abort(err)
		}
		m.mu.Lock()
		runtime.limbo = limboLease
		runtime.state = roomRunning
		m.mu.Unlock()
	} else {
		m.mu.Lock()
		runtime.state = roomRunning
		m.mu.Unlock()
	}
	if ctx.Err() != nil {
		return abort(ctx.Err())
	}
	go func() {
		defer m.finishRuntime(runtime)
		m.run(ctx, index, profile, token)
	}()
	return nil
}

// prepareProfile ensures the profile's pack directory exists and performs the
// automatic pull at most once per profile until the config changes.
func (m *Manager) prepareProfile(ctx context.Context, profile config.Profile) error {
	dir, err := EnsureLimboPacksDir(m.dir, profile)
	if err != nil {
		return err
	}
	if !profile.LimboAutoPullPacks {
		return nil
	}
	m.mu.Lock()
	prepared := m.preparedPacks[profile.ID]
	m.mu.Unlock()
	if prepared {
		return nil
	}
	lock := m.prepLock(profile.ID)
	lock.Lock()
	defer lock.Unlock()
	m.mu.Lock()
	prepared = m.preparedPacks[profile.ID]
	m.mu.Unlock()
	if prepared {
		return nil
	}

	name, uid := m.packIdentity(profile.ID)
	output, pullErr := m.pullFn(ctx, profile, name, uid)
	if pullErr != nil {
		if ctx.Err() == nil {
			m.record("资源包", fmt.Sprintf("配置方案「%s」自动拉取后端资源包失败：%s", profile.Name, pullErr.Error()),
				false, 0, profile.Name)
		}
		return nil
	}
	if output != "" && m.logger != nil {
		m.logger.Printf("[packs][%s] %s", profile.Name, output)
	}
	if removed, warnings, dedupeErr := dedupeResourcePacks(dir); dedupeErr != nil {
		m.record("资源包", fmt.Sprintf("配置方案「%s」清理重复资源包失败：%s", profile.Name, dedupeErr.Error()),
			false, 0, profile.Name)
	} else if len(removed) > 0 {
		m.record("资源包", fmt.Sprintf("配置方案「%s」已删除 %d 个重复资源包：%s",
			profile.Name, len(removed), strings.Join(removed, ", ")), false, 0, profile.Name)
	} else {
		for _, warning := range warnings {
			m.record("资源包", warning, false, 0, profile.Name)
		}
	}
	m.mu.Lock()
	m.preparedPacks[profile.ID] = true
	m.mu.Unlock()
	m.record("资源包", fmt.Sprintf("配置方案「%s」已从后端服拉取资源包", profile.Name), false, 0, profile.Name)
	return nil
}

// packIdentity picks an account assigned to the profile for pack pulls, falling
// back to the shared bot identity when the profile is unused.
func (m *Manager) packIdentity(profileID string) (string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fallback := m.cfg.DefaultProfileID()
	for _, record := range m.accounts {
		id := record.ProfileID
		if id == "" {
			id = fallback
		}
		if id != profileID {
			continue
		}
		name := strings.TrimSpace(record.Name)
		uid := strings.TrimSpace(record.UID)
		if name != "" || uid != "" {
			if name == "" {
				name = "CubeDockPackBot"
			}
			if uid == "" {
				uid = "1000000000"
			}
			return name, uid
		}
	}
	return "CubeDockPackBot", "1000000000"
}

// StopAccount asks one room to shut down without affecting other rooms.
func (m *Manager) StopAccount(index int) error {
	m.mu.Lock()
	runtime := m.runtimes[index]
	if runtime == nil {
		m.mu.Unlock()
		return nil
	}
	if runtime.state == roomStopping {
		m.mu.Unlock()
		return nil
	}
	runtime.state = roomStopping
	runtime.cancel()
	if index >= 0 && index < len(m.status.Rooms) {
		m.status.Rooms[index].Phase = "stopping"
	}
	m.mu.Unlock()
	m.persist()
	return nil
}

// Stop asks every room to shut down.
func (m *Manager) Stop() {
	m.mu.Lock()
	for index, runtime := range m.runtimes {
		runtime.state = roomStopping
		runtime.cancel()
		if index >= 0 && index < len(m.status.Rooms) {
			m.status.Rooms[index].Phase = "stopping"
		}
	}
	m.mu.Unlock()
	m.stopAllLimbos()
	m.persist()
}

// RestartAccount stops one room, waits for its resources to be released and
// then starts it again. Other rooms are untouched.
func (m *Manager) RestartAccount(index int) error {
	lock := m.actionLock(index)
	lock.Lock()
	defer lock.Unlock()

	m.mu.Lock()
	runtime := m.runtimes[index]
	if runtime == nil {
		m.mu.Unlock()
		return m.startAccount(index)
	}
	runtime.state = roomStopping
	runtime.cancel()
	if index >= 0 && index < len(m.status.Rooms) {
		m.status.Rooms[index].Phase = "stopping"
	}
	done := runtime.done
	m.mu.Unlock()
	m.persist()

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		return fmt.Errorf("房间停止超时，请稍后重试")
	}
	return m.startAccount(index)
}

func (m *Manager) waitRuntime(index int, timeout time.Duration) bool {
	m.mu.Lock()
	runtime := m.runtimes[index]
	if runtime == nil {
		m.mu.Unlock()
		return true
	}
	done := runtime.done
	m.mu.Unlock()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// finishRuntime removes one runtime exactly once, releases its lease and limbo
// reference, then closes its done channel so RestartAccount can reclaim the
// account safely.
func (m *Manager) finishRuntime(runtime *roomRuntime) {
	runtime.finishOnce.Do(func() {
		m.mu.Lock()
		if current, ok := m.runtimes[runtime.index]; ok && current == runtime {
			delete(m.runtimes, runtime.index)
		}
		m.syncRoomsLocked()
		if runtime.index >= 0 && runtime.index < len(m.status.Rooms) {
			status := &m.status.Rooms[runtime.index]
			status.Phase = "stopped"
			status.RoomID = 0
			status.Players = 0
			status.ProtocolID = 0
			status.UpstreamOnline = false
			status.UpstreamVersion = ""
			status.ConnectedAt = ""
			status.RetryAt = ""
			status.Attempts = 0
			status.LastError = ""
		}
		if len(m.runtimes) == 0 {
			close(m.idle)
		}
		m.mu.Unlock()

		runtime.cancel()
		runtime.lease.Release()
		if runtime.limbo != nil {
			runtime.limbo.Release()
		}
		close(runtime.done)
		m.persist()
		m.record("系统", fmt.Sprintf("房间 %d 已停止", runtime.index+1), false,
			runtime.index+1, runtime.profile.Name)
	})
}

// Wait blocks until no room runtime remains.
func (m *Manager) Wait() {
	m.mu.Lock()
	idle := m.idle
	m.mu.Unlock()
	<-idle
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func playerLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "未知玩家"
	}
	return name
}

func shortDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d 秒", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d 分 %d 秒", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d 时 %d 分", int(d.Hours()), int(d.Minutes())%60)
}

// roomOccupancy returns the current guest count for console messages. The host
// bot occupies one slot, so the guest capacity is maxPlayers-1.
func (m *Manager) roomOccupancy(index, maxPlayers int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.status.Rooms) {
		return ""
	}
	return fmt.Sprintf("（房间在线 %d/%d）", m.status.Rooms[index].Players, maxPlayers-1)
}

type roomLog struct {
	manager *Manager
	index   int
	profile string
}

func (l roomLog) event(message string) {
	l.manager.record(fmt.Sprintf("房间 %d", l.index+1), message, false, l.index+1, l.profile)
}

func (l roomLog) debug(message string) {
	l.manager.record(fmt.Sprintf("房间 %d", l.index+1), message, true, l.index+1, l.profile)
}
