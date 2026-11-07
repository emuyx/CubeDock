package secret

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "room-tokens.dat")
	if err := Save(path, "first-token\nsecond-token"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" && string(raw) == "first-token\nsecond-token" {
		t.Fatal("secret was written in plaintext despite DPAPI being available")
	}
	value, err := Load(path)
	if err != nil || value != "first-token\nsecond-token" {
		t.Fatalf("round trip = %q, %v", value, err)
	}
	if value, err := Load(filepath.Join(t.TempDir(), "missing")); err != nil || value != "" {
		t.Fatalf("missing file = %q, %v", value, err)
	}
	if Description() == "" {
		t.Fatal("missing storage description")
	}
}
