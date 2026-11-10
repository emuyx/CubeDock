package fileutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReplaceWritesFileAndCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	if err := Replace(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Replace(path, []byte("second"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "second" {
		t.Fatalf("content = %q, %v", raw, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "state.json" {
		t.Fatalf("temporary file left behind: %v", entries)
	}
}

func TestCleanupStaleState(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, ".state-old")
	recentPath := filepath.Join(dir, ".state-recent")
	keepPath := filepath.Join(dir, "state.json")
	for _, path := range []string{oldPath, recentPath, keepPath} {
		if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldPath, old, old); err != nil {
		t.Fatal(err)
	}
	if removed := CleanupStaleState(dir, time.Hour); removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("stale state file was kept")
	}
	for _, path := range []string{recentPath, keepPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s was removed: %v", path, err)
		}
	}
}
