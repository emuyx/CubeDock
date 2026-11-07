package core

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emuyx/cubedock/internal/relay"
	"github.com/emuyx/cubedock/internal/relay/relaytest"
)

func addTestPlayer(manager *Manager, room int, name string) *livePlayer {
	player := manager.players.register(room, uint32(room+100), "account", func() {})
	manager.players.noteLogin(player, name)
	manager.players.notePlayStatus(player, 0)
	return player
}

func TestRunCommandHelpAndErrors(t *testing.T) {
	manager := newTestManager(t)

	if _, err := manager.RunCommand("   "); err == nil {
		t.Fatal("empty command accepted")
	}
	if _, err := manager.RunCommand("bogus"); err == nil || !strings.Contains(err.Error(), "未知命令") {
		t.Fatalf("unknown command = %v", err)
	}
	if _, err := manager.RunCommand("help extra"); err == nil {
		t.Fatal("help accepted an argument")
	}
	if _, err := manager.RunCommand("list extra"); err == nil {
		t.Fatal("list accepted an argument")
	}
	if _, err := manager.RunCommand(strings.Repeat("x", maxConsoleCommandRunes+1)); err == nil || !strings.Contains(err.Error(), "过长") {
		t.Fatalf("long command = %v", err)
	}

	result, err := manager.RunCommand("HELP")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(result.Output, "\n")
	for _, expected := range []string{"help", "list", "kick", "Alice Smith"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("help missing %q:\n%s", expected, joined)
		}
	}
}

func TestRunCommandList(t *testing.T) {
	manager := newTestManager(t)
	result, err := manager.RunCommand("list")
	if err != nil || len(result.Output) != 1 || !strings.Contains(result.Output[0], "没有玩家") {
		t.Fatalf("empty list = %+v, %v", result, err)
	}

	addTestPlayer(manager, 0, "Alice Smith")
	addTestPlayer(manager, 1, "Bob")
	result, err = manager.RunCommand("list")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(result.Output, "\n")
	for _, expected := range []string{"p1", "Alice Smith", "房间 1", "p2", "Bob", "房间 2", "在线", "上行 0 包"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("list missing %q:\n%s", expected, joined)
		}
	}
}

func TestRunCommandKick(t *testing.T) {
	t.Run("by id", func(t *testing.T) {
		manager := newTestManager(t)
		player := addTestPlayer(manager, 0, "Alice")
		result, err := manager.RunCommand("kick p1")
		if err != nil || !strings.Contains(strings.Join(result.Output, "\n"), "已请求断开") {
			t.Fatalf("kick = %+v, %v", result, err)
		}
		if player.state != PlayerKicked {
			t.Fatalf("state = %q", player.state)
		}
	})

	for _, command := range []string{
		`kick "Alice Smith"`,
		`kick 'Alice Smith'`,
		`kick Alice Smith`,
		`kick Alice\ Smith`,
	} {
		t.Run(command, func(t *testing.T) {
			manager := newTestManager(t)
			addTestPlayer(manager, 0, "Alice Smith")
			if _, err := manager.RunCommand(command); err != nil {
				t.Fatalf("%s = %v", command, err)
			}
		})
	}

	for _, command := range []string{
		`kick "Alice Smith`,
		`kick "Alice" extra`,
	} {
		t.Run(command, func(t *testing.T) {
			manager := newTestManager(t)
			addTestPlayer(manager, 0, "Alice Smith")
			if _, err := manager.RunCommand(command); err == nil {
				t.Fatalf("%s accepted", command)
			}
		})
	}

	t.Run("missing target", func(t *testing.T) {
		manager := newTestManager(t)
		if _, err := manager.RunCommand("kick"); err == nil {
			t.Fatal("kick without target accepted")
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		manager := newTestManager(t)
		addTestPlayer(manager, 0, "Alice")
		addTestPlayer(manager, 1, "alice")
		if _, err := manager.RunCommand("kick Alice"); err == nil ||
			!strings.Contains(err.Error(), "p1@房间 1") || !strings.Contains(err.Error(), "p2@房间 2") {
			t.Fatalf("ambiguous kick = %v", err)
		}
	})

	t.Run("audit event", func(t *testing.T) {
		manager := newTestManager(t)
		addTestPlayer(manager, 0, "Alice")
		if _, err := manager.RunCommand("kick p1"); err != nil {
			t.Fatal(err)
		}
		events := manager.Snapshot().Events
		if len(events) == 0 || !strings.Contains(events[len(events)-1].Message, "管理员已断开玩家 Alice") {
			t.Fatalf("events = %+v", events)
		}
	})
}

func TestRunCommandTransfer(t *testing.T) {
	manager := newTestManager(t)
	player := addTestPlayer(manager, 0, "Alice Smith")
	stream := relaytest.New()
	manager.players.attachClient(player, &relay.SyncStream{Stream: stream})

	result, err := manager.RunCommand(`transfer "Alice Smith" 192.0.2.10:19132`)
	if err != nil || !strings.Contains(strings.Join(result.Output, "\n"), "转服包") {
		t.Fatalf("transfer = %+v, %v", result, err)
	}
	batch, err := stream.Next(time.Second)
	if err != nil {
		t.Fatalf("transfer packet: %v", err)
	}
	if len(batch) == 0 || batch[0] != 0xff {
		t.Fatalf("transfer batch = %x", batch)
	}

	for _, command := range []string{
		"transfer",
		"transfer Alice",
		"transfer p1 not-a-host-port",
		"transfer missing 192.0.2.10:19132",
	} {
		if _, err := manager.RunCommand(command); err == nil {
			t.Fatalf("%s accepted", command)
		}
	}
}

func TestRunCommandKickCancel(t *testing.T) {
	manager := newTestManager(t)
	var cancelled atomic.Int32
	player := manager.players.register(0, 1, "account", func() { cancelled.Add(1) })
	manager.players.noteLogin(player, "Alice")

	if _, err := manager.RunCommand("kick Alice"); err != nil {
		t.Fatal(err)
	}
	if cancelled.Load() != 1 {
		t.Fatalf("cancel calls = %d", cancelled.Load())
	}
	if _, err := manager.RunCommand("kick p1"); err == nil || !strings.Contains(err.Error(), "正在断开") {
		t.Fatalf("double kick = %v", err)
	}
}
