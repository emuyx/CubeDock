package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// pullResourcePacks runs the bundled limbo in one-shot pull mode. limbo logs
// into the backend server as a bot, downloads unencrypted resource packs, and
// exits. It is shared by automatic pulls and the console refresh action.
func (m *Manager) pullResourcePacks(ctx context.Context) (string, error) {
	m.mu.Lock()
	cfg := m.cfg
	name, uid := "CubeDockPackBot", "1000000000"
	if len(m.accounts) > 0 {
		if strings.TrimSpace(m.accounts[0].Name) != "" {
			name = m.accounts[0].Name
		}
		if strings.TrimSpace(m.accounts[0].UID) != "" {
			uid = m.accounts[0].UID
		}
	}
	m.mu.Unlock()

	path, err := limboExecutable(cfg)
	if err != nil {
		return "", err
	}
	packsDir, err := EnsureLimboPacksDir(cfg)
	if err != nil {
		return "", err
	}
	args := []string{
		"-pull",
		"-target", cfg.Upstream,
		"-packs", packsDir,
		"-identity-name", name,
		"-identity-uid", uid,
	}
	output, err := exec.CommandContext(ctx, path, args...).CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text != "" {
			return text, fmt.Errorf("拉取资源包失败: %w（%s）", err, text)
		}
		return "", fmt.Errorf("拉取资源包失败: %w", err)
	}
	return text, nil
}
