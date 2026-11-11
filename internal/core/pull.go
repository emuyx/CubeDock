package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/emuyx/cubedock/internal/config"
)

// pullResourcePacks runs the bundled limbo in one-shot pull mode. limbo logs
// into the backend server as a bot, downloads unencrypted resource packs, and
// exits. It is shared by automatic pulls and the console refresh action.
func (m *Manager) pullResourcePacks(ctx context.Context, profile config.Profile, name, uid string) (string, error) {
	m.mu.Lock()
	limboPath := m.cfg.LimboPath
	m.mu.Unlock()

	path, err := limboExecutable(limboPath)
	if err != nil {
		return "", err
	}
	packsDir, err := EnsureLimboPacksDir(m.dir, profile)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(name) == "" {
		name = "CubeDockPackBot"
	}
	if strings.TrimSpace(uid) == "" {
		uid = "1000000000"
	}
	args := []string{
		"-pull",
		"-target", profile.Upstream,
		"-packs", packsDir,
		"-identity-name", name,
		"-identity-uid", uid,
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = childEnv()
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil {
		if text != "" {
			return text, fmt.Errorf("拉取资源包失败: %w（%s）", err, text)
		}
		return "", fmt.Errorf("拉取资源包失败: %w", err)
	}
	return text, nil
}
