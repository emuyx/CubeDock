package core

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/session"
)

type fakeOpener struct {
	mu       sync.Mutex
	hosts    []*fakeRoomHost
	profiles map[string]config.Profile
}

func (o *fakeOpener) open(_ context.Context, profile config.Profile, info auth.RoomInfo, _ func(string)) (roomHost, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.profiles == nil {
		o.profiles = make(map[string]config.Profile)
	}
	o.profiles[info.PlayerName] = profile
	host := newFakeRoomHost(uint32(900000 + len(o.hosts)))
	o.hosts = append(o.hosts, host)
	return host, nil
}

func (o *fakeOpener) snapshot() ([]*fakeRoomHost, map[string]config.Profile) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]*fakeRoomHost(nil), o.hosts...), copyProfiles(o.profiles)
}

func copyProfiles(input map[string]config.Profile) map[string]config.Profile {
	out := make(map[string]config.Profile, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

type fakeLimboFactory struct {
	mu      sync.Mutex
	started []config.Profile
	stopped atomic.Int32
}

func (f *fakeLimboFactory) start(_ string, profile config.Profile, _ string, _ *log.Logger, _ func(string)) (*limboProcess, error) {
	f.mu.Lock()
	f.started = append(f.started, profile)
	f.mu.Unlock()
	done := make(chan struct{})
	var once sync.Once
	return &limboProcess{done: done, cleanup: func() {
		once.Do(func() {
			f.stopped.Add(1)
			close(done)
		})
	}}, nil
}

func twoProfileConfig() config.Config {
	cfg := config.Defaults()
	cfg.Profiles[0].AutoAcceptFriends = false
	alt := config.DefaultProfile()
	alt.ID = "alt"
	alt.Name = "备用方案"
	alt.Upstream = "127.0.0.1:19134"
	alt.RoomName = "备用房间"
	alt.AutoAcceptFriends = false
	alt.LimboUpstream = "127.0.0.1:19134"
	cfg.Profiles = append(cfg.Profiles, alt)
	return cfg
}

func installFakeRoomSeams(t *testing.T, manager *Manager, lockDir string) *fakeOpener {
	t.Helper()
	manager.claimSession = func(token string) (*session.Lease, error) {
		return session.ClaimIn(lockDir, token)
	}
	manager.pingFn = func(_ context.Context, profile config.Profile) (string, error) {
		return "1.21.130.0", nil
	}
	manager.prepareRoom = func(_ context.Context, _ uint8, token string) (auth.RoomInfo, error) {
		return auth.RoomInfo{UserID: 1, PlayerName: "bot-" + token, ProtocolID: 43}, nil
	}
	opener := &fakeOpener{}
	manager.openRoom = opener.open
	return opener
}

func TestStartUsesPerAccountProfiles(t *testing.T) {
	manager := newTestManager(t)
	if err := manager.Configure(twoProfileConfig()); err != nil {
		t.Fatal(err)
	}
	addTestAccounts(t, manager, 3)
	if err := manager.AssignAccountProfile(1, "alt"); err != nil {
		t.Fatal(err)
	}
	opener := installFakeRoomSeams(t, manager, t.TempDir())

	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool {
		rooms := manager.Snapshot().Rooms
		if len(rooms) != 3 {
			return false
		}
		for _, room := range rooms {
			if room.Phase != "online" {
				return false
			}
		}
		return true
	}, "rooms did not reach online")

	snapshot := manager.Snapshot()
	if snapshot.Rooms[0].ProfileID != "default" || snapshot.Rooms[1].ProfileID != "alt" ||
		snapshot.Rooms[2].ProfileID != "default" {
		t.Fatalf("room profiles = %+v", snapshot.Rooms)
	}
	if snapshot.Rooms[1].ProfileName != "备用方案" || snapshot.Rooms[1].ConnectionMode != config.ConnectionProxy {
		t.Fatalf("resolved room profile = %+v", snapshot.Rooms[1])
	}
	_, profiles := opener.snapshot()
	if profiles["bot-token-01"].ID != "alt" || profiles["bot-token-00"].ID != "default" {
		t.Fatalf("openRoom profiles = %+v", profiles)
	}

	manager.Stop()
	waitForStop(t, manager)
}

func TestStartAndStopAccountIsolation(t *testing.T) {
	manager := newTestManager(t)
	addTestAccounts(t, manager, 2)
	opener := installFakeRoomSeams(t, manager, t.TempDir())

	if err := manager.StartAccount(0); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool {
		rooms := manager.Snapshot().Rooms
		return len(rooms) == 2 && rooms[0].Phase == "online" && rooms[1].Phase == "stopped"
	}, "starting one account affected the other")

	if err := manager.StartAccount(1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool {
		rooms := manager.Snapshot().Rooms
		return rooms[0].Phase == "online" && rooms[1].Phase == "online"
	}, "second account did not come online")

	if err := manager.StopAccount(0); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool {
		rooms := manager.Snapshot().Rooms
		return rooms[0].Phase == "stopped" && rooms[1].Phase == "online"
	}, "stopping one account affected the other")

	hosts, _ := opener.snapshot()
	if len(hosts) != 2 || hosts[0].closed.Load() != 1 || hosts[1].closed.Load() != 0 {
		t.Fatalf("hosts after stopping room 0: %d opened, closed=%d/%d",
			len(hosts), hosts[0].closed.Load(), hosts[1].closed.Load())
	}

	if err := manager.RestartAccount(1); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool {
		rooms := manager.Snapshot().Rooms
		return rooms[0].Phase == "stopped" && rooms[1].Phase == "online"
	}, "restart did not restore only the target room")
	hosts, _ = opener.snapshot()
	if len(hosts) != 3 || hosts[1].closed.Load() != 1 || hosts[2].closed.Load() != 0 {
		t.Fatalf("hosts after restart: %d opened, closed=%d/%d", len(hosts),
			hosts[1].closed.Load(), hosts[2].closed.Load())
	}

	manager.Stop()
	waitForStop(t, manager)
}

func TestTransferProfileStartsOneLimbo(t *testing.T) {
	manager := newTestManager(t)
	cfg := config.Defaults()
	cfg.Profiles[0].ConnectionMode = config.ConnectionTransfer
	cfg.Profiles[0].LimboAutoPullPacks = false
	cfg.Profiles[0].AutoAcceptFriends = false
	if err := manager.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	addTestAccounts(t, manager, 2)
	installFakeRoomSeams(t, manager, t.TempDir())
	factory := &fakeLimboFactory{}
	manager.startLimboFn = factory.start
	manager.pingFn = func(context.Context, config.Profile) (string, error) { return "limbo", nil }

	if err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, func() bool {
		rooms := manager.Snapshot().Rooms
		return len(rooms) == 2 && rooms[0].Phase == "online" && rooms[1].Phase == "online"
	}, "transfer rooms did not reach online")

	factory.mu.Lock()
	started := len(factory.started)
	factory.mu.Unlock()
	if started != 1 {
		t.Fatalf("limbo starts = %d, want 1 for one shared profile", started)
	}

	manager.Stop()
	waitForStop(t, manager)
	if factory.stopped.Load() != 1 {
		t.Fatalf("limbo stops = %d, want 1", factory.stopped.Load())
	}
}

func TestAssignAccountProfilePersists(t *testing.T) {
	manager := newTestManager(t)
	if err := manager.Configure(twoProfileConfig()); err != nil {
		t.Fatal(err)
	}
	addTestAccounts(t, manager, 1)
	if err := manager.AssignAccountProfile(0, "alt"); err != nil {
		t.Fatal(err)
	}
	if err := manager.AssignAccountProfile(0, "missing"); err == nil {
		t.Fatal("unknown profile accepted")
	}
	if accounts := manager.Accounts(); accounts[0].ProfileID != "alt" || accounts[0].ProfileName != "备用方案" {
		t.Fatalf("account profile = %+v", accounts[0])
	}
	again, err := New(manager.Dir(), twoProfileConfig(), []string{"token-00"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if accounts := again.Accounts(); accounts[0].ProfileID != "alt" {
		t.Fatalf("reloaded account profile = %+v", accounts[0])
	}
}

func TestLoadAccountsBackfillsDefaultProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")
	if err := os.WriteFile(path, []byte(`[{"email":"old@example.com","name":"Old"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := New(dir, config.Defaults(), []string{"token-old"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	accounts := manager.Accounts()
	if len(accounts) != 1 || accounts[0].ProfileID != config.DefaultProfileID {
		t.Fatalf("accounts = %+v", accounts)
	}
}
