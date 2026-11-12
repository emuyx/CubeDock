package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/netease"
	"github.com/emuyx/cubedock/internal/room"
	"github.com/emuyx/cubedock/internal/session"
)

// fakeRoomHost blocks Accept until Close, which is enough to exercise the
// manager's startup, supervision and shutdown paths without NetEase.
type fakeRoomHost struct {
	id      uint32
	done    chan struct{}
	once    sync.Once
	closed  atomic.Int32
	kicked  atomic.Uint32
	members []room.RoomMember
}

func newFakeRoomHost(id uint32) *fakeRoomHost {
	return &fakeRoomHost{id: id, done: make(chan struct{})}
}

func (h *fakeRoomHost) RoomID() uint32            { return h.id }
func (h *fakeRoomHost) Done() <-chan struct{}     { return h.done }
func (h *fakeRoomHost) Err() error                { return nil }
func (h *fakeRoomHost) Close()                    { h.once.Do(func() { close(h.done); h.closed.Store(1) }) }
func (h *fakeRoomHost) Accept() (net.Conn, error) { <-h.done; return nil, net.ErrClosed }
func (h *fakeRoomHost) PlayerIDFor(net.Conn) uint32 {
	return 0
}
func (h *fakeRoomHost) KickPlayer(playerID uint32) error {
	h.kicked.Store(playerID)
	return nil
}
func (h *fakeRoomHost) RoomMembers() []room.RoomMember { return h.members }

func addTestAccounts(t *testing.T, manager *Manager, count int) {
	t.Helper()
	for index := 0; index < count; index++ {
		token := fmt.Sprintf("token-%02d", index)
		if _, err := manager.AddAccount(token, AccountMeta{
			Email: fmt.Sprintf("user%02d@example.com", index),
			Name:  fmt.Sprintf("User%02d", index),
		}); err != nil {
			t.Fatalf("add account %d: %v", index, err)
		}
	}
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal(message)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForStop(t *testing.T, manager *Manager) {
	t.Helper()
	stopped := make(chan struct{})
	go func() { manager.Wait(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("rooms did not stop")
	}
}

// TestManagerSupervisesSixteenRooms starts more rooms than the old fixed cap
// and checks that every lease, host and goroutine is released on Stop.
func TestManagerSupervisesSixteenRooms(t *testing.T) {
	const count = 16
	manager := newTestManager(t)
	cfg := config.Defaults()
	cfg.Profiles[0].AutoAcceptFriends = false
	if err := manager.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	addTestAccounts(t, manager, count)

	lockDir := t.TempDir()
	manager.claimSession = func(token string) (*session.Lease, error) {
		return session.ClaimIn(lockDir, token)
	}
	manager.pingFn = func(context.Context, config.Profile) (string, error) {
		return "1.21.130.0", nil
	}
	manager.prepareRoom = func(_ context.Context, _ netease.AuthMethod, _ uint8, token string) (auth.RoomInfo, error) {
		return auth.RoomInfo{UserID: 1, PlayerName: "bot-" + token, ProtocolID: 43}, nil
	}

	var openedMu sync.Mutex
	opened := make([]*fakeRoomHost, 0, count)
	manager.openRoom = func(context.Context, config.Profile, auth.RoomInfo, func(string)) (roomHost, error) {
		openedMu.Lock()
		defer openedMu.Unlock()
		host := newFakeRoomHost(uint32(900000 + len(opened)))
		opened = append(opened, host)
		return host, nil
	}

	baseline := runtime.NumGoroutine()
	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool {
		rooms := manager.Snapshot().Rooms
		if len(rooms) != count {
			return false
		}
		for _, room := range rooms {
			if room.Phase != "online" {
				return false
			}
		}
		return true
	}, "not all rooms reached online")

	openedMu.Lock()
	if len(opened) != count {
		openedMu.Unlock()
		t.Fatalf("opened %d hosts, want %d", len(opened), count)
	}
	for _, host := range opened {
		if host.closed.Load() != 0 {
			openedMu.Unlock()
			t.Fatal("host closed while the room should be online")
		}
	}
	openedMu.Unlock()

	manager.Stop()
	waitForStop(t, manager)
	if manager.Snapshot().Running {
		t.Fatal("manager still running after Stop")
	}
	if rooms := manager.Snapshot().Rooms; len(rooms) != count {
		t.Fatalf("rooms after Stop = %d, want %d", len(rooms), count)
	} else {
		for index, room := range rooms {
			if room.Phase != "stopped" {
				t.Fatalf("room %d phase after Stop = %q", index, room.Phase)
			}
		}
	}

	openedMu.Lock()
	for index, host := range opened {
		if host.closed.Load() == 0 {
			openedMu.Unlock()
			t.Fatalf("host %d was not closed", index)
		}
	}
	openedMu.Unlock()

	for index := 0; index < count; index++ {
		token := fmt.Sprintf("token-%02d", index)
		lease, err := session.ClaimIn(lockDir, token)
		if err != nil {
			t.Fatalf("lease for %s not released: %v", token, err)
		}
		lease.Release()
	}

	waitFor(t, 3*time.Second, func() bool {
		return runtime.NumGoroutine() <= baseline+5
	}, "goroutines did not return to the baseline after Stop")
}

// TestManagerReleasesLeasesWhenAccountIsBusy checks that one busy account
// aborts the whole start and every lease taken before it is released.
func TestManagerReleasesLeasesWhenAccountIsBusy(t *testing.T) {
	const count = 10
	manager := newTestManager(t)
	addTestAccounts(t, manager, count)

	lockDir := t.TempDir()
	manager.claimSession = func(token string) (*session.Lease, error) {
		if token == "token-07" {
			return nil, session.ErrTokenBusy
		}
		return session.ClaimIn(lockDir, token)
	}
	manager.pingFn = func(context.Context, config.Profile) (string, error) {
		return "1.21.130.0", nil
	}

	err := manager.Start()
	if err == nil {
		t.Fatal("start succeeded with a busy account")
	}
	if !errors.Is(err, session.ErrTokenBusy) {
		t.Fatalf("start error = %v, want ErrTokenBusy", err)
	}
	if !strings.Contains(err.Error(), "房间 8") {
		t.Fatalf("start error lacks room label: %v", err)
	}
	if manager.Snapshot().Running {
		t.Fatal("manager marked running after a rejected start")
	}
	if rooms := manager.Snapshot().Rooms; len(rooms) != count {
		t.Fatalf("rooms after rejected start = %d, want %d", len(rooms), count)
	} else {
		for index, room := range rooms {
			if room.Phase != "stopped" {
				t.Fatalf("room %d phase after rejected start = %q", index, room.Phase)
			}
		}
	}

	for index := 0; index < 7; index++ {
		token := fmt.Sprintf("token-%02d", index)
		lease, claimErr := session.ClaimIn(lockDir, token)
		if claimErr != nil {
			t.Fatalf("lease for %s not rolled back: %v", token, claimErr)
		}
		lease.Release()
	}
}

// TestManagerStopCancelsBlockedAuthentication ensures Stop unblocks room
// goroutines stuck in the NetEase authentication step.
func TestManagerStopCancelsBlockedAuthentication(t *testing.T) {
	const count = 9
	manager := newTestManager(t)
	addTestAccounts(t, manager, count)

	lockDir := t.TempDir()
	manager.claimSession = func(token string) (*session.Lease, error) {
		return session.ClaimIn(lockDir, token)
	}
	manager.pingFn = func(context.Context, config.Profile) (string, error) {
		return "1.21.130.0", nil
	}
	manager.prepareRoom = func(ctx context.Context, _ netease.AuthMethod, _ uint8, _ string) (auth.RoomInfo, error) {
		<-ctx.Done()
		return auth.RoomInfo{}, ctx.Err()
	}

	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Second, func() bool {
		rooms := manager.Snapshot().Rooms
		if len(rooms) != count {
			return false
		}
		for _, room := range rooms {
			if room.Phase != "authenticating" {
				return false
			}
		}
		return true
	}, "rooms did not reach the authentication step")

	manager.Stop()
	waitForStop(t, manager)

	for index := 0; index < count; index++ {
		token := fmt.Sprintf("token-%02d", index)
		lease, err := session.ClaimIn(lockDir, token)
		if err != nil {
			t.Fatalf("lease for %s not released after Stop: %v", token, err)
		}
		lease.Release()
	}
}

// TestManagerKeepsHealthyRoomsAfterOneFailure checks that a room that cannot
// open does not take down the others and enters the retry phase.
func TestManagerKeepsHealthyRoomsAfterOneFailure(t *testing.T) {
	const count = 10
	manager := newTestManager(t)
	cfg := config.Defaults()
	cfg.Profiles[0].AutoAcceptFriends = false
	if err := manager.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	addTestAccounts(t, manager, count)

	manager.claimSession = func(token string) (*session.Lease, error) {
		return session.ClaimIn(t.TempDir(), token)
	}
	manager.pingFn = func(context.Context, config.Profile) (string, error) {
		return "1.21.130.0", nil
	}
	manager.prepareRoom = func(_ context.Context, _ netease.AuthMethod, _ uint8, token string) (auth.RoomInfo, error) {
		return auth.RoomInfo{UserID: 1, PlayerName: "bot-" + token, ProtocolID: 43}, nil
	}
	manager.openRoom = func(_ context.Context, _ config.Profile, info auth.RoomInfo, _ func(string)) (roomHost, error) {
		if info.PlayerName == "bot-token-03" {
			return nil, errors.New("room node rejected")
		}
		return newFakeRoomHost(1), nil
	}

	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool {
		rooms := manager.Snapshot().Rooms
		if len(rooms) != count {
			return false
		}
		online := 0
		for _, room := range rooms {
			if room.Phase == "online" {
				online++
			}
		}
		return online == count-1 && rooms[3].Phase == "retrying"
	}, "healthy rooms or the failing room did not reach the expected phase")

	rooms := manager.Snapshot().Rooms
	if !strings.Contains(rooms[3].LastError, "room node rejected") {
		t.Fatalf("failed room error = %q", rooms[3].LastError)
	}

	manager.Stop()
	waitForStop(t, manager)
}
