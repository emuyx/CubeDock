package fileutil

import (
	"os"
	"path/filepath"
	"testing"
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
