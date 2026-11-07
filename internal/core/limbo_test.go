package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emuyx/cubedock/internal/config"
)

func TestEnsureLimboPacksDirCreatesExplicitDirectory(t *testing.T) {
	cfg := config.Defaults()
	cfg.LimboPacksDir = filepath.Join(t.TempDir(), "explicit-packs")

	resolved, err := EnsureLimboPacksDir(cfg)
	if err != nil {
		t.Fatalf("EnsureLimboPacksDir: %v", err)
	}
	if resolved != cfg.LimboPacksDir {
		t.Fatalf("resolved = %q, want %q", resolved, cfg.LimboPacksDir)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		t.Fatalf("stat packs dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", resolved)
	}
}

func TestDefaultLimboPacksDirFollowsExecutable(t *testing.T) {
	base := t.TempDir()
	cfg := config.Defaults()
	cfg.LimboPath = filepath.Join(base, "limbo.exe")

	want := filepath.Join(base, "packs")
	if got := DefaultLimboPacksDir(cfg); got != want {
		t.Fatalf("DefaultLimboPacksDir = %q, want %q", got, want)
	}
}
