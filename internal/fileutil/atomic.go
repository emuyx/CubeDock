// Package fileutil provides crash-safe replacement of small state files.
package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Replace writes data through a temporary file in the destination directory and
// renames it into place, so readers never observe a partially written file.
func Replace(path string, data []byte, perm os.FileMode) error {
	folder := filepath.Dir(path)
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return fmt.Errorf("创建状态目录: %w", err)
	}

	handle, err := os.CreateTemp(folder, ".state-*")
	if err != nil {
		return fmt.Errorf("创建临时状态文件: %w", err)
	}
	tempPath := handle.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()

	if err := writeState(handle, data, perm); err != nil {
		_ = handle.Close()
		return err
	}
	if err := handle.Close(); err != nil {
		return fmt.Errorf("关闭临时状态文件: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("替换状态文件: %w", err)
	}
	committed = true
	return nil
}

// CleanupStaleState removes leftover ".state-*" temporary files older than
// age. A crash between write and rename can otherwise leave a plaintext copy
// of a token or status file behind indefinitely.
func CleanupStaleState(dir string, age time.Duration) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	cutoff := time.Now().Add(-age)
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), ".state-") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || !info.ModTime().Before(cutoff) {
			continue
		}
		if os.Remove(filepath.Join(dir, entry.Name())) == nil {
			removed++
		}
	}
	return removed
}

func writeState(handle *os.File, data []byte, perm os.FileMode) error {
	if err := handle.Chmod(perm); err != nil {
		return fmt.Errorf("设置状态文件权限: %w", err)
	}
	if _, err := handle.Write(data); err != nil {
		return fmt.Errorf("写入状态文件: %w", err)
	}
	if err := handle.Sync(); err != nil {
		return fmt.Errorf("同步状态文件: %w", err)
	}
	return nil
}
