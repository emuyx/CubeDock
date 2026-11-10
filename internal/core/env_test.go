package core

import (
	"strings"
	"testing"
)

func TestChildEnvDropsBootstrapSecrets(t *testing.T) {
	t.Setenv("CUBEDOCK_TOKEN", "token-value")
	t.Setenv("CUBEDOCK_ADMIN_PASSWORD", "password-value")
	t.Setenv("CUBEDOCK_KEEP_ME", "keep")

	foundKeep := false
	for _, entry := range childEnv() {
		key, _, _ := strings.Cut(entry, "=")
		if isBootstrapSecret(key) {
			t.Fatalf("bootstrap secret passed to child: %s", key)
		}
		if entry == "CUBEDOCK_KEEP_ME=keep" {
			foundKeep = true
		}
	}
	if !foundKeep {
		t.Fatal("unrelated environment variable was dropped")
	}
}
