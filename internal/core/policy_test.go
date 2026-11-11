package core

import (
	"testing"

	"github.com/emuyx/cubedock/internal/config"
)

func TestAutoAcceptFriendsHonoursFriendsOnly(t *testing.T) {
	profile := config.DefaultProfile()
	profile.AutoAcceptFriends = true
	profile.FriendsOnly = false
	if !shouldAutoAcceptFriends(profile) {
		t.Fatal("auto-accept disabled for an open room")
	}

	profile.FriendsOnly = true
	if shouldAutoAcceptFriends(profile) {
		t.Fatal("auto-accept enabled for a friends-only room")
	}

	profile.AutoAcceptFriends = false
	if shouldAutoAcceptFriends(profile) {
		t.Fatal("auto-accept enabled while disabled by the operator")
	}
}
