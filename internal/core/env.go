package core

import (
	"os"
	"strings"
)

// bootstrapSecrets are read once from the environment at startup. Helpers such
// as limbo and the pack puller never need them, so they are stripped from the
// environment those child processes inherit.
var bootstrapSecrets = []string{"CUBEDOCK_TOKEN", "CUBEDOCK_ADMIN_PASSWORD"}

func isBootstrapSecret(key string) bool {
	for _, secret := range bootstrapSecrets {
		// Windows environment variable names are case-insensitive.
		if strings.EqualFold(key, secret) {
			return true
		}
	}
	return false
}

// childEnv returns the current environment without the bootstrap secrets.
func childEnv() []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if isBootstrapSecret(key) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
