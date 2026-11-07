// Package config loads CubeDock's non-secret settings.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/emuyx/cubedock/internal/fileutil"
)

const (
	// ConnectionProxy relays every player's game traffic through CubeDock.
	ConnectionProxy = "proxy"
	// ConnectionTransfer uses the bundled limbo helper: StartGame + empty chunks,
	// then Transfer to the configured upstream after horizontal movement.
	ConnectionTransfer = "transfer"
)

// Config is the persisted non-secret configuration. Account credentials and
// device keys live in separately protected files.
type Config struct {
	ConnectionMode     string `json:"connection_mode"`
	LimboUpstream      string `json:"limbo_upstream"`
	LimboPath          string `json:"limbo_path,omitempty"`
	LimboPacksDir      string `json:"limbo_packs_dir,omitempty"`
	LimboAutoPullPacks bool   `json:"limbo_auto_pull_packs"`
	LimboFormTitle     string `json:"limbo_form_title,omitempty"`
	LimboFormContent   string `json:"limbo_form_content,omitempty"`
	LimboFormButton    string `json:"limbo_form_button,omitempty"`
	LobbyProtocolID    uint8  `json:"lobby_protocol_id"`
	Upstream           string `json:"upstream"`
	RoomName           string `json:"room_name"`
	RoomGameVersion    string `json:"room_game_version"`
	RoomPassword       string `json:"room_password"`
	MaxPlayers         int    `json:"max_players"`
	FriendsOnly        bool   `json:"friends_only"`
	AutoAcceptFriends  bool   `json:"auto_accept_friends"`
	AllowPvP           bool   `json:"allow_pvp"`
	AutoStart          bool   `json:"auto_start"`
	WebPort            int    `json:"web_port"`
	RetrySeconds       int    `json:"retry_seconds"`
	Debug              bool   `json:"debug"`
}

// Defaults returns a usable configuration for a fresh data directory.
func Defaults() Config {
	return Config{
		ConnectionMode:     ConnectionProxy,
		LimboUpstream:      "127.0.0.1:19133",
		LimboAutoPullPacks: true,
		LimboFormTitle:     "CubeDock Limbo",
		LimboFormContent:   "点击下方按钮进入服务器。",
		LimboFormButton:    "进入服务器",
		LobbyProtocolID:    43,
		Upstream:           "127.0.0.1:19132",
		RoomName:           "来和我一起玩吧！",
		RoomGameVersion:    "1.21.130.0",
		MaxPlayers:         10,
		AutoAcceptFriends:  true,
		AllowPvP:           true,
		WebPort:            18321,
		RetrySeconds:       10,
	}
}

// Normalize fills fields that older config files may omit and clamps values
// that CubeDock cannot honour. It reports whether the file should be rewritten.
func (c *Config) Normalize() bool {
	changed := false
	if c.ConnectionMode == "" {
		c.ConnectionMode = ConnectionProxy
		changed = true
	}
	if c.LimboUpstream == "" {
		c.LimboUpstream = "127.0.0.1:19133"
		changed = true
	}
	if c.LobbyProtocolID == 0 {
		c.LobbyProtocolID = 43
		changed = true
	}
	if strings.TrimSpace(c.LimboFormTitle) == "" {
		c.LimboFormTitle = "CubeDock Limbo"
		changed = true
	}
	if strings.TrimSpace(c.LimboFormContent) == "" {
		c.LimboFormContent = "点击下方按钮进入服务器。"
		changed = true
	}
	if strings.TrimSpace(c.LimboFormButton) == "" {
		c.LimboFormButton = "进入服务器"
		changed = true
	}
	// NetEase grants at most ten slots; old builds allowed requesting forty.
	if c.MaxPlayers > 10 && c.MaxPlayers <= 40 {
		c.MaxPlayers = 10
		changed = true
	}
	return changed
}

// Validate rejects settings that would fail only after opening a room.
func (c Config) Validate() error {
	switch c.ConnectionMode {
	case ConnectionProxy, ConnectionTransfer:
	default:
		return fmt.Errorf("连接模式必须为 proxy 或 transfer")
	}
	if c.ConnectionMode == ConnectionTransfer {
		limboHost, limboPort, err := net.SplitHostPort(c.LimboUpstream)
		if err != nil || strings.TrimSpace(limboHost) == "" {
			return fmt.Errorf("Limbo 地址格式应为 IP:端口")
		}
		if port, err := strconv.Atoi(limboPort); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("Limbo 端口无效")
		}
	}
	host, port, err := net.SplitHostPort(c.Upstream)
	if err != nil || strings.TrimSpace(host) == "" {
		return fmt.Errorf("服务器地址格式应为 IP:端口")
	}
	upstreamPort, err := strconv.Atoi(port)
	if err != nil || upstreamPort < 1 || upstreamPort > 65535 {
		return fmt.Errorf("服务器端口无效")
	}
	if c.MaxPlayers < 2 || c.MaxPlayers > 10 {
		return fmt.Errorf("房间总人数必须为 2–10（含房主机器人）")
	}
	if name := len([]rune(c.RoomName)); name < 1 || name > 40 {
		return fmt.Errorf("房间名称必须为 1–40 字")
	}
	if len(c.RoomPassword) > 64 {
		return fmt.Errorf("房间密码过长")
	}
	if len([]rune(c.LimboFormTitle)) > 64 {
		return fmt.Errorf("转服表单标题最多 64 字")
	}
	if len([]rune(c.LimboFormContent)) > 2000 {
		return fmt.Errorf("转服表单内容最多 2000 字")
	}
	if len([]rune(c.LimboFormButton)) > 64 {
		return fmt.Errorf("转服按钮文字最多 64 字")
	}
	version := strings.Split(c.RoomGameVersion, ".")
	if len(version) != 4 {
		return fmt.Errorf("网易房间内核版本应为四段数字，例如 1.21.130.0")
	}
	for _, part := range version {
		if _, err := strconv.ParseUint(part, 10, 16); err != nil {
			return fmt.Errorf("网易房间内核版本格式无效")
		}
	}
	if c.WebPort < 1024 || c.WebPort > 65535 {
		return fmt.Errorf("控制台端口必须为 1024–65535")
	}
	if c.RetrySeconds < 5 || c.RetrySeconds > 300 {
		return fmt.Errorf("重连间隔必须为 5–300 秒")
	}
	return nil
}

// Load reads config.json, creating it with defaults when absent. Unknown keys
// from older deployments are ignored and dropped on the next save.
func Load(path string) (Config, error) {
	c := Defaults()
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, Save(path, c)
	}
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	// 3.10 clients use the 1.21.130 room version and lobby protocol 43.
	// Migrate the previous defaults on load, but keep Normalize free to
	// preserve explicit API/UI values.
	if c.RoomGameVersion == "1.21.120.0" {
		c.RoomGameVersion = "1.21.130.0"
	}
	if c.LobbyProtocolID == 42 {
		c.LobbyProtocolID = 43
	}
	c.Normalize()
	if err = c.Validate(); err != nil {
		return c, err
	}
	// Rewrite once when removed or legacy keys are present, so the on-disk
	// file matches what CubeDock actually uses.
	normalized, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return c, err
	}
	normalized = append(normalized, '\n')
	if !bytes.Equal(bytes.TrimSpace(raw), bytes.TrimSpace(normalized)) {
		if err = fileutil.Replace(path, normalized, 0600); err != nil {
			return c, err
		}
	}
	return c, nil
}

// Save validates and atomically writes the configuration.
func Save(path string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.Replace(path, append(raw, '\n'), 0600)
}
