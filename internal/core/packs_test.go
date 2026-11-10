package core

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testManifest = `{
  "format_version": 2,
  "header": {
    "name": "Test Pack",
    "description": "test",
    "uuid": "11111111-2222-3333-4444-555555555555",
    "version": [1, 2, 3]
  },
  "modules": [
    {"type": "resources", "uuid": "66666666-7777-8888-9999-000000000000", "version": [1, 2, 3]}
  ]
}`

func TestScanResourcePacks(t *testing.T) {
	dir := t.TempDir()
	if err := writeTestPackZip(filepath.Join(dir, "a.mcpack"), testManifest); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "folder-pack")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "manifest.json"), []byte(testManifest), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.mcpack"), []byte("not a zip"), 0600); err != nil {
		t.Fatal(err)
	}

	list, err := scanResourcePacks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if list.Dir != dir {
		t.Fatalf("dir = %q", list.Dir)
	}
	if len(list.Packs) != 3 {
		t.Fatalf("packs = %d, want 3", len(list.Packs))
	}
	byFile := make(map[string]ResourcePack, len(list.Packs))
	for _, pack := range list.Packs {
		byFile[pack.File] = pack
	}
	for _, name := range []string{"a.mcpack", "folder-pack"} {
		pack := byFile[name]
		if pack.Name != "Test Pack" || pack.UUID != "11111111-2222-3333-4444-555555555555" ||
			pack.Version != "1.2.3" || pack.Error != "" || pack.Size == 0 {
			t.Fatalf("%s pack = %+v", name, pack)
		}
	}
	if broken := byFile["broken.mcpack"]; !strings.Contains(broken.Error, "不是有效的") {
		t.Fatalf("broken pack error = %q", broken.Error)
	}
}

func TestScanResourcePacksEmpty(t *testing.T) {
	list, err := scanResourcePacks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Packs) != 0 {
		t.Fatalf("packs = %d, want 0", len(list.Packs))
	}
}

func TestDedupeResourcePacks(t *testing.T) {
	dir := t.TempDir()
	// Both duplicate files use CubeDock's managed naming shape; the newer one
	// is the canonical <manifest-uuid>_<version>.mcpack and must be kept.
	older := filepath.Join(dir, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee_1.2.3.mcpack")
	newer := filepath.Join(dir, "11111111-2222-3333-4444-555555555555_1.2.3.mcpack")
	conflict := filepath.Join(dir, "conflict.mcpack")
	if err := writeTestPackZip(older, testManifest); err != nil {
		t.Fatal(err)
	}
	if err := writeTestPackZip(newer, testManifest); err != nil {
		t.Fatal(err)
	}
	conflicting := strings.Replace(testManifest, `"name": "Test Pack"`, `"name": "Other Pack"`, 1)
	if err := writeTestPackZip(conflict, conflicting); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()
	_ = os.Chtimes(older, oldTime, oldTime)
	_ = os.Chtimes(newer, newTime, newTime)
	_ = os.Chtimes(conflict, newTime, newTime)

	removed, warnings, err := dedupeResourcePacks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != filepath.Base(older) {
		t.Fatalf("removed = %v", removed)
	}
	if len(warnings) == 0 {
		t.Fatal("conflicting content was not reported")
	}
	for _, path := range []string{newer, conflict} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s should remain: %v", path, err)
		}
	}
}

func TestDedupeKeepsUnmanagedFiles(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "custom-a.mcpack")
	second := filepath.Join(dir, "custom-b.mcpack")
	for _, path := range []string{first, second} {
		if err := writeTestPackZip(path, testManifest); err != nil {
			t.Fatal(err)
		}
	}
	removed, _, err := dedupeResourcePacks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Fatalf("unmanaged files were deleted: %v", removed)
	}
	for _, path := range []string{first, second} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s should remain: %v", path, err)
		}
	}
}

func writeTestPackZip(path, manifest string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("manifest.json")
	if err != nil {
		_ = file.Close()
		return err
	}
	if _, err = entry.Write([]byte(manifest)); err != nil {
		_ = file.Close()
		return err
	}
	if err = writer.Close(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
