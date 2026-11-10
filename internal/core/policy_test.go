package core

import (
	"testing"

	"github.com/emuyx/cubedock/internal/config"
)

func TestAutoAcceptFriendsHonoursFriendsOnly(t *testing.T) {
	cfg := config.Defaults()
	cfg.AutoAcceptFriends = true
	cfg.FriendsOnly = false
	if !shouldAutoAcceptFriends(cfg) {
		t.Fatal("auto-accept disabled for an open room")
	}

	cfg.FriendsOnly = true
	if shouldAutoAcceptFriends(cfg) {
		t.Fatal("auto-accept enabled for a friends-only room")
	}

	cfg.AutoAcceptFriends = false
	if shouldAutoAcceptFriends(cfg) {
		t.Fatal("auto-accept enabled while disabled by the operator")
	}
}
