package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emuyx/cubedock/internal/config"
)

func TestEnsureLimboPacksDir(t *testing.T) {
	cfg := config.Defaults()
	cfg.LimboPacksDir = filepath.Join(t.TempDir(), "custom-packs")
	dir, err := EnsureLimboPacksDir(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if dir != cfg.LimboPacksDir {
		t.Fatalf("dir = %q, want %q", dir, cfg.LimboPacksDir)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("packs dir not created: %v", err)
	}
}

func TestDefaultLimboPacksDirUsesLimboPath(t *testing.T) {
	base := t.TempDir()
	cfg := config.Defaults()
	cfg.LimboPath = filepath.Join(base, "limbo.exe")
	if got := DefaultLimboPacksDir(cfg); got != filepath.Join(base, "packs") {
		t.Fatalf("dir = %q", got)
	}
}
