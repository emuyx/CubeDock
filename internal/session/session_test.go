package session

import (
	"errors"
	"testing"
)

func TestClaimIsExclusivePerToken(t *testing.T) {
	dir := t.TempDir()
	first, err := ClaimIn(dir, "account-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimIn(dir, "account-a"); !errors.Is(err, ErrTokenBusy) {
		t.Fatalf("second claim = %v", err)
	}
	second, err := ClaimIn(dir, "account-b")
	if err != nil {
		t.Fatalf("different token blocked: %v", err)
	}
	first.Release()
	first.Release() // idempotent
	reclaimed, err := ClaimIn(dir, "account-a")
	if err != nil {
		t.Fatalf("released token still blocked: %v", err)
	}
	second.Release()
	reclaimed.Release()
}

func TestClaimRejectsEmptyToken(t *testing.T) {
	if _, err := ClaimIn(t.TempDir(), "   "); err == nil {
		t.Fatal("empty token accepted")
	}
}
