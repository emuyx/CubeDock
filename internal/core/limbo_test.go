package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emuyx/cubedock/internal/config"
)

func TestEnsureLimboPacksDirCreatesExplicitDirectory(t *testing.T) {
	profile := config.DefaultProfile()
	profile.LimboPacksDir = filepath.Join(t.TempDir(), "explicit-packs")

	resolved, err := EnsureLimboPacksDir(t.TempDir(), profile)
	if err != nil {
		t.Fatalf("EnsureLimboPacksDir: %v", err)
	}
	if resolved != profile.LimboPacksDir {
		t.Fatalf("resolved = %q, want %q", resolved, profile.LimboPacksDir)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		t.Fatalf("stat packs dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", resolved)
	}
}

func TestProfilePacksDirIsolatedByProfile(t *testing.T) {
	dataDir := t.TempDir()
	first := config.DefaultProfile()
	first.ID = "survival"
	second := config.DefaultProfile()
	second.ID = "creative"

	want := filepath.Join(dataDir, "packs", "survival")
	if got := ProfilePacksDir(dataDir, first); got != want {
		t.Fatalf("ProfilePacksDir = %q, want %q", got, want)
	}
	if ProfilePacksDir(dataDir, first) == ProfilePacksDir(dataDir, second) {
		t.Fatal("different profiles share one packs directory")
	}
}
