//go:build !windows

// Package secret stores credentials protected by the operating system.
package secret

import (
	"errors"
	"os"

	"github.com/emuyx/cubedock/internal/fileutil"
)

// Description explains where the secret is kept.
func Description() string { return "0600 文件（仅当前用户可读写）" }

// Save writes value with owner-only permissions.
func Save(path, value string) error {
	return fileutil.Replace(path, []byte(value), 0o600)
}

// Load returns a saved value; a missing file yields an empty string.
func Load(path string) (string, error) {
	raw, err := readOwnerFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func readOwnerFile(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return raw, err
}
