package core

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/emuyx/cubedock/internal/relay"
	"github.com/emuyx/cubedock/internal/relay/relaytest"
)

func TestPlayerRegistryLifecycle(t *testing.T) {
	registry := &playerRegistry{}
	var up, down atomic.Uint64
	var cancelled atomic.Int32

	player := registry.register(0, 12345, "AliceAccount", func() { cancelled.Add(1) })
	if player.id != "p1" {
		t.Fatalf("first id = %q", player.id)
	}
	registry.noteLogin(player, "Alice")
	registry.notePlayStatus(player, 0)
	registry.attachStreams(player, &up, &down)
	up.Add(5)
	down.Add(7)

	players := registry.list()
	if len(players) != 1 {
		t.Fatalf("list = %+v", players)
	}
	got := players[0]
	if got.ID != "p1" || got.Name != "Alice" || got.Room != 1 || got.RoomID != 12345 ||
		got.Account != "AliceAccount" || got.State != PlayerJoined ||
		got.UpPackets != 5 || got.DownPackets != 7 || got.JoinedAt == "" {
		t.Fatalf("status = %+v", got)
	}

	registry.unregister(player)
	if players := registry.list(); len(players) != 0 {
		t.Fatalf("unregistered player still listed: %+v", players)
	}
	next := registry.register(1, 9000, "BobAccount", func() { cancelled.Add(1) })
	if next.id != "p2" {
		t.Fatalf("id was reused: %q", next.id)
	}
	if cancelled.Load() != 0 {
		t.Fatal("cancel called outside kick")
	}
}

func TestPlayerRegistryStateTransitions(t *testing.T) {
	registry := &playerRegistry{}
	player := registry.register(0, 1, "account", func() {})

	if got := registry.list()[0].State; got != PlayerConnecting {
		t.Fatalf("initial state = %q", got)
	}
	registry.noteLogin(player, "Alice")
	if got := registry.list()[0].State; got != PlayerAuthenticating {
		t.Fatalf("after login = %q", got)
	}
	registry.notePlayStatus(player, 7)
	if got := registry.list()[0].State; got != PlayerLoginFailed {
		t.Fatalf("after failure = %q", got)
	}

	registry.noteLogin(player, "Alice")
	if _, _, err := registry.kick("Alice"); err != nil {
		t.Fatal(err)
	}
	registry.noteLogin(player, "Changed")
	registry.notePlayStatus(player, 0)
	got := registry.list()[0]
	if got.State != PlayerKicked || got.Name != "Alice" {
		t.Fatalf("late hook overwrote kicked state: %+v", got)
	}
}

func TestPlayerRegistryKickByNameIDAndAmbiguity(t *testing.T) {
	registry := &playerRegistry{}
	var cancelled atomic.Int32
	cancel := func() { cancelled.Add(1) }

	alice := registry.register(0, 1, "a", cancel)
	registry.noteLogin(alice, "Alice")
	registry.notePlayStatus(alice, 0)

	aliceSmith := registry.register(0, 1, "a", cancel)
	registry.noteLogin(aliceSmith, "Alice Smith")
	registry.notePlayStatus(aliceSmith, 0)

	otherAlice := registry.register(1, 2, "b", cancel)
	registry.noteLogin(otherAlice, "alice")
	registry.notePlayStatus(otherAlice, 0)

	if _, _, err := registry.kick("Alice"); err == nil ||
		!strings.Contains(err.Error(), "p1@房间 1") || !strings.Contains(err.Error(), "p3@房间 2") {
		t.Fatalf("ambiguous name error = %v", err)
	}

	status, kickCancel, err := registry.kick("Alice Smith")
	if kickCancel != nil {
		kickCancel()
	}
	if err != nil || status.ID != "p2" || cancelled.Load() != 1 {
		t.Fatalf("kick spaced name = %+v, %v, cancel=%d", status, err, cancelled.Load())
	}

	status, _, err = registry.kick("P1")
	if err != nil || status.ID != "p1" {
		t.Fatalf("kick id = %+v, %v", status, err)
	}
	if _, _, err := registry.kick("p1"); err == nil || !strings.Contains(err.Error(), "正在断开") {
		t.Fatalf("double kick error = %v", err)
	}
	if _, _, err := registry.kick("p99"); err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("missing id error = %v", err)
	}
	if _, _, err := registry.kick("Nobody"); err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("missing name error = %v", err)
	}
}

func TestPlayerRegistryTransferRequiresJoined(t *testing.T) {
	registry := &playerRegistry{}
	player := registry.register(0, 1, "account", func() {})
	registry.noteLogin(player, "Alice")
	stream := relaytest.New()
	registry.attachClient(player, &relay.SyncStream{Stream: stream})

	if _, _, err := registry.transferTarget("Alice"); err == nil || !strings.Contains(err.Error(), "尚未完成登录") {
		t.Fatalf("transfer before join = %v", err)
	}
	registry.notePlayStatus(player, 0)
	status, client, err := registry.transferTarget("p1")
	if err != nil || status.ID != "p1" || client == nil {
		t.Fatalf("transfer target = %+v, %v, %v", status, client, err)
	}
}

func TestPlayerRegistryUnregisterOnlyRemovesSamePlayer(t *testing.T) {
	registry := &playerRegistry{}
	first := registry.register(0, 1, "a", func() {})
	second := registry.register(0, 1, "a", func() {})
	registry.unregister(first)
	registry.unregister(first)
	if players := registry.list(); len(players) != 1 || players[0].ID != second.id {
		t.Fatalf("stale unregister removed another player: %+v", players)
	}
}

func TestPlayerRegistryConcurrentSnapshots(t *testing.T) {
	registry := &playerRegistry{}
	var up, down atomic.Uint64
	player := registry.register(0, 1, "a", func() {})
	registry.attachStreams(player, &up, &down)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				up.Add(1)
				down.Add(1)
				_ = registry.list()
				registry.noteLogin(player, "Alice")
			}
		}()
	}
	wg.Wait()
	got := registry.list()[0]
	if got.UpPackets != 800 || got.DownPackets != 800 {
		t.Fatalf("counters = %d/%d", got.UpPackets, got.DownPackets)
	}
}
