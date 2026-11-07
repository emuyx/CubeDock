// Package fileutil provides crash-safe replacement of small state files.
package fileutil

import (
	"os"
	"path/filepath"
)

// Replace atomically replaces path with data. The temporary file is created
// in the destination directory, so the final rename never crosses a volume.
func Replace(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".cubedock-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)

	if err = file.Chmod(perm); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}
