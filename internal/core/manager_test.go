package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emuyx/cubedock/internal/config"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	manager, err := New(t.TempDir(), config.Defaults(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func TestAddAccountHasNoFixedLimit(t *testing.T) {
	manager := newTestManager(t)
	const count = 12
	for index := 0; index < count; index++ {
		token := fmt.Sprintf("token-%02d", index)
		got, err := manager.AddAccount(token, AccountMeta{
			Email: fmt.Sprintf("user%02d@example.com", index),
			Name:  fmt.Sprintf("User%02d", index),
		})
		if err != nil {
			t.Fatalf("add account %d: %v", index, err)
		}
		if got != index {
			t.Fatalf("add account %d returned index %d", index, got)
		}
	}
	if accounts := manager.Accounts(); len(accounts) != count {
		t.Fatalf("accounts = %d, want %d", len(accounts), count)
	}
	for index := 0; index < count; index++ {
		token, err := manager.Token(index)
		if err != nil || token != fmt.Sprintf("token-%02d", index) {
			t.Fatalf("token %d = %q, %v", index, token, err)
		}
	}
}

func TestManagerAccountLifecycle(t *testing.T) {
	manager := newTestManager(t)
	if err := manager.Start(); err == nil || !strings.Contains(err.Error(), "添加网易账号") {
		t.Fatalf("start without accounts = %v", err)
	}

	index, err := manager.AddAccount("token-a", AccountMeta{
		Email: "a@example.com", Label: "A", Name: "Alice", UID: "1", Level: 3,
		Platform: "interop", ExpiresAt: "2026-11-01T00:00:00Z",
	})
	if err != nil || index != 0 {
		t.Fatalf("add = %d, %v", index, err)
	}
	if _, err := manager.AddAccount("token-a", AccountMeta{Email: "dup@example.com"}); err == nil {
		t.Fatal("duplicate session accepted")
	}
	if index, err := manager.AddAccount("token-b", AccountMeta{Email: "b@example.com", Name: "Bob"}); err != nil || index != 1 {
		t.Fatalf("second add = %d, %v", index, err)
	}
	accounts := manager.Accounts()
	if len(accounts) != 2 || accounts[0].Name != "Alice" || accounts[0].Level != 3 ||
		accounts[1].Email != "b@example.com" {
		t.Fatalf("accounts = %+v", accounts)
	}
	token, err := manager.Token(0)
	if err != nil || token != "token-a" {
		t.Fatalf("token = %q, %v", token, err)
	}

	if err := manager.UpdateAccountProfile(1, "Bobby", "2", 4, "https://example.invalid/avatar.png", ""); err != nil {
		t.Fatal(err)
	}
	if err := manager.RemoveAccount(0); err != nil {
		t.Fatal(err)
	}
	accounts = manager.Accounts()
	if len(accounts) != 1 || accounts[0].Name != "Bobby" || accounts[0].Level != 4 ||
		accounts[0].AvatarURL != "https://example.invalid/avatar.png" {
		t.Fatalf("after remove = %+v", accounts)
	}
	if token, err := manager.Token(0); err != nil || token != "token-b" {
		t.Fatalf("remaining token = %q, %v", token, err)
	}
	if err := manager.RemoveAccount(5); err == nil {
		t.Fatal("out-of-range removal accepted")
	}

	// Metadata must survive a restart with the same token list.
	again, err := New(manager.Dir(), config.Defaults(), []string{"token-b"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	accounts = again.Accounts()
	if len(accounts) != 1 || accounts[0].Name != "Bobby" {
		t.Fatalf("reloaded accounts = %+v", accounts)
	}
	for _, name := range []string{"room-tokens.dat", "accounts.json", "logs/status.json"} {
		if _, err := os.Stat(filepath.Join(manager.Dir(), name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestSetDebugControlsTraceEvents(t *testing.T) {
	manager := newTestManager(t)
	manager.trace("系统", "关闭时不应出现")
	if events := manager.Snapshot().Events; len(events) != 0 {
		t.Fatalf("trace emitted while debug off: %+v", events)
	}

	if err := manager.SetDebug(true); err != nil {
		t.Fatal(err)
	}
	manager.trace("系统", "开启后应出现")
	found := false
	for _, event := range manager.Snapshot().Events {
		if event.Level == "debug" && strings.Contains(event.Message, "开启后应出现") {
			found = true
		}
	}
	if !found {
		t.Fatal("trace event missing after enabling debug")
	}
	loaded, err := config.Load(filepath.Join(manager.Dir(), "config.json"))
	if err != nil || !loaded.Debug {
		t.Fatalf("debug not persisted: %v, %v", loaded.Debug, err)
	}

	if err := manager.SetDebug(false); err != nil {
		t.Fatal(err)
	}
	before := len(manager.Snapshot().Events)
	manager.trace("系统", "关闭后不应出现")
	if after := len(manager.Snapshot().Events); after != before {
		t.Fatalf("trace emitted after disabling debug: %d -> %d", before, after)
	}
}

func TestStartTransferRequiresLimboHelper(t *testing.T) {
	manager := newTestManager(t)
	next := config.Defaults()
	next.Profiles[0].ConnectionMode = config.ConnectionTransfer
	next.LimboPath = filepath.Join(t.TempDir(), "missing-limbo")
	if err := manager.Configure(next); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.AddAccount("token-transfer", AccountMeta{Email: "transfer@example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(); err == nil || !strings.Contains(strings.ToLower(err.Error()), "limbo") {
		t.Fatalf("transfer start without limbo = %v", err)
	}
	if manager.Snapshot().Running {
		t.Fatal("manager marked running after rejected transfer start")
	}
}

func TestManagerConfigure(t *testing.T) {
	manager := newTestManager(t)
	next := config.Defaults()
	next.WebPort = 19000
	if err := manager.Configure(next); err == nil {
		t.Fatal("web port change accepted at runtime")
	}
	next = config.Defaults()
	next.Profiles[0].RoomName = "新房间"
	if err := manager.Configure(next); err != nil {
		t.Fatal(err)
	}
	if got := manager.Snapshot().Config.Profiles[0].RoomName; got != "新房间" {
		t.Fatalf("room name = %q", got)
	}
	bad := config.Defaults()
	bad.Profiles[0].Upstream = "bad"
	if err := manager.Configure(bad); err == nil {
		t.Fatal("invalid configuration accepted")
	}
}

func TestConfigureRevisionRejectsStaleWrites(t *testing.T) {
	manager := newTestManager(t)
	base := manager.Snapshot().ConfigRevision

	first := manager.Snapshot().Config.Clone()
	first.Profiles[0].RoomName = "第一次修改"
	if err := manager.ConfigureIfRevision(first, base); err != nil {
		t.Fatal(err)
	}
	if got := manager.Snapshot().ConfigRevision; got != base+1 {
		t.Fatalf("revision = %d, want %d", got, base+1)
	}

	stale := manager.Snapshot().Config.Clone()
	stale.Profiles[0].RoomName = "过期覆盖"
	if err := manager.ConfigureIfRevision(stale, base); !errors.Is(err, ErrConfigChanged) {
		t.Fatalf("stale configure error = %v, want ErrConfigChanged", err)
	}
	if got := manager.Snapshot().Config.Profiles[0].RoomName; got != "第一次修改" {
		t.Fatalf("stale write changed config: %q", got)
	}

	current := manager.Snapshot()
	if _, err := manager.ConfigureForRestoreIfRevision(current.Config.Clone(), base); !errors.Is(err, ErrConfigChanged) {
		t.Fatalf("stale restore error = %v, want ErrConfigChanged", err)
	}
	restored := current.Config.Clone()
	restored.Profiles[0].RoomName = "恢复名称"
	if _, err := manager.ConfigureForRestoreIfRevision(restored, current.ConfigRevision); err != nil {
		t.Fatal(err)
	}
	if got := manager.Snapshot().ConfigRevision; got != current.ConfigRevision+1 {
		t.Fatalf("restore revision = %d, want %d", got, current.ConfigRevision+1)
	}
}

func TestSetDebugBumpsConfigRevision(t *testing.T) {
	manager := newTestManager(t)
	before := manager.Snapshot().ConfigRevision
	if err := manager.SetDebug(true); err != nil {
		t.Fatal(err)
	}
	if got := manager.Snapshot().ConfigRevision; got != before+1 {
		t.Fatalf("revision = %d, want %d", got, before+1)
	}
	if !manager.Snapshot().Config.Debug {
		t.Fatal("debug flag not persisted")
	}
}

func TestAccountPasswordStorageAndRemoval(t *testing.T) {
	manager := newTestManager(t)
	if _, err := manager.AddAccount("token-password", AccountMeta{
		Email: "password@example.com", Name: "Password", UID: "9001",
	}); err != nil {
		t.Fatal(err)
	}
	if err := manager.SaveAccountPassword(0, "local-secret"); err != nil {
		t.Fatal(err)
	}
	password, saved, err := manager.AccountPassword(0)
	if err != nil || !saved || password != "local-secret" {
		t.Fatalf("password = %q, saved=%v, err=%v", password, saved, err)
	}
	if err := manager.SaveAccountPassword(0, ""); err != nil {
		t.Fatal(err)
	}
	if _, saved, err := manager.AccountPassword(0); err != nil || saved {
		t.Fatalf("password should be removed, saved=%v, err=%v", saved, err)
	}
	if err := manager.SaveAccountPassword(0, "remove-with-account"); err != nil {
		t.Fatal(err)
	}
	if err := manager.RemoveAccount(0); err != nil {
		t.Fatal(err)
	}
	if _, saved, err := manager.AccountPassword(0); err == nil || saved {
		t.Fatalf("removed account password lookup: saved=%v err=%v", saved, err)
	}
}
