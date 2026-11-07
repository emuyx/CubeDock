//go:build !windows

// Package secret stores credentials protected by the operating system.
package secret

import (
	"os"

	"github.com/emuyx/cubedock/internal/fileutil"
)

// Description explains where the secret is kept.
func Description() string { return "0600 文件（仅当前用户可读写）" }

// Save writes value with owner-only permissions.
func Save(path, value string) error { return fileutil.Replace(path, []byte(value), 0600) }

// Load returns a saved value; a missing file yields an empty string.
func Load(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(raw), err
}
