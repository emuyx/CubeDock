// Package config loads CubeDock's non-secret settings.
//
// A Config holds installation-wide settings plus one or more named room/server
// profiles. Every NetEase account selects one profile, so different accounts can
// host rooms against different upstream servers with different room settings.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
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

	// DefaultProfileID is the stable ID used by migrated installations and as
	// the fallback for accounts that do not select a profile.
	DefaultProfileID = "default"
	// DefaultProfileName is the display name of the migrated profile.
	DefaultProfileName = "默认配置"

	maxProfiles = 32
)

var profileIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Profile is one named set of upstream, limbo and room settings.
type Profile struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	ConnectionMode     string `json:"connection_mode"`
	Upstream           string `json:"upstream"`
	LimboUpstream      string `json:"limbo_upstream"`
	LimboPacksDir      string `json:"limbo_packs_dir,omitempty"`
	LimboAutoPullPacks bool   `json:"limbo_auto_pull_packs"`
	LimboFormTitle     string `json:"limbo_form_title,omitempty"`
	LimboFormContent   string `json:"limbo_form_content,omitempty"`
	LimboFormButton    string `json:"limbo_form_button,omitempty"`
	LobbyProtocolID    uint8  `json:"lobby_protocol_id"`
	RoomName           string `json:"room_name"`
	RoomGameVersion    string `json:"room_game_version"`
	RoomPassword       string `json:"room_password"`
	MaxPlayers         int    `json:"max_players"`
	FriendsOnly        bool   `json:"friends_only"`
	AutoAcceptFriends  bool   `json:"auto_accept_friends"`
	AllowPvP           bool   `json:"allow_pvp"`
	RetrySeconds       int    `json:"retry_seconds"`
}

// Config is the persisted non-secret configuration. Account credentials and
// device keys live in separately protected files.
type Config struct {
	Profiles  []Profile `json:"profiles"`
	LimboPath string    `json:"limbo_path,omitempty"`
	AutoStart bool      `json:"auto_start"`
	WebPort   int       `json:"web_port"`
	Debug     bool      `json:"debug"`
}

// DefaultProfile returns the profile used by a fresh installation.
func DefaultProfile() Profile {
	return Profile{
		ID:                 DefaultProfileID,
		Name:               DefaultProfileName,
		ConnectionMode:     ConnectionProxy,
		Upstream:           "127.0.0.1:19132",
		LimboUpstream:      "127.0.0.1:19133",
		LimboAutoPullPacks: true,
		LimboFormTitle:     "CubeDock Limbo",
		LimboFormContent:   "点击下方按钮进入服务器。",
		LimboFormButton:    "进入服务器",
		LobbyProtocolID:    43,
		RoomName:           "来和我一起玩吧！",
		RoomGameVersion:    "1.21.130.0",
		MaxPlayers:         10,
		AutoAcceptFriends:  true,
		AllowPvP:           true,
		RetrySeconds:       10,
	}
}

// Defaults returns a usable configuration for a fresh data directory.
func Defaults() Config {
	return Config{
		Profiles: []Profile{DefaultProfile()},
		WebPort:  18321,
	}
}

// Normalize fills fields that older or hand-written files may omit. It reports
// whether anything changed.
func (p *Profile) Normalize(index int) bool {
	changed := false
	if strings.TrimSpace(p.ID) == "" {
		p.ID = fmt.Sprintf("profile-%d", index+1)
		changed = true
	}
	if strings.TrimSpace(p.Name) == "" {
		p.Name = fmt.Sprintf("配置方案 %d", index+1)
		changed = true
	}
	if p.ConnectionMode == "" {
		p.ConnectionMode = ConnectionProxy
		changed = true
	}
	if strings.TrimSpace(p.Upstream) == "" {
		p.Upstream = "127.0.0.1:19132"
		changed = true
	}
	if strings.TrimSpace(p.LimboUpstream) == "" {
		p.LimboUpstream = fmt.Sprintf("127.0.0.1:%d", 19133+index)
		changed = true
	}
	if p.LobbyProtocolID == 0 {
		p.LobbyProtocolID = 43
		changed = true
	}
	if strings.TrimSpace(p.RoomName) == "" {
		p.RoomName = "来和我一起玩吧！"
		changed = true
	}
	if strings.TrimSpace(p.RoomGameVersion) == "" {
		p.RoomGameVersion = "1.21.130.0"
		changed = true
	}
	if p.MaxPlayers == 0 {
		p.MaxPlayers = 10
		changed = true
	}
	// NetEase grants at most ten slots; old builds allowed requesting forty.
	if p.MaxPlayers > 10 && p.MaxPlayers <= 40 {
		p.MaxPlayers = 10
		changed = true
	}
	if p.RetrySeconds == 0 {
		p.RetrySeconds = 10
		changed = true
	}
	if strings.TrimSpace(p.LimboFormTitle) == "" {
		p.LimboFormTitle = "CubeDock Limbo"
		changed = true
	}
	if strings.TrimSpace(p.LimboFormContent) == "" {
		p.LimboFormContent = "点击下方按钮进入服务器。"
		changed = true
	}
	if strings.TrimSpace(p.LimboFormButton) == "" {
		p.LimboFormButton = "进入服务器"
		changed = true
	}
	return changed
}

// Normalize fills installation-wide defaults and normalizes every profile.
func (c *Config) Normalize() bool {
	changed := false
	for index := range c.Profiles {
		if c.Profiles[index].Normalize(index) {
			changed = true
		}
	}
	if c.WebPort == 0 {
		c.WebPort = 18321
		changed = true
	}
	return changed
}

// Validate rejects profile settings that would fail only after opening a room.
func (p Profile) Validate() error {
	if !profileIDPattern.MatchString(p.ID) {
		return fmt.Errorf("配置方案 ID 只能包含字母、数字、下划线和连字符，最多 64 个字符")
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("配置方案名称不能为空")
	}
	if len([]rune(p.Name)) > 64 {
		return fmt.Errorf("配置方案名称最多 64 个字符")
	}
	switch p.ConnectionMode {
	case ConnectionProxy, ConnectionTransfer:
	default:
		return fmt.Errorf("连接模式必须为 proxy 或 transfer")
	}
	if p.ConnectionMode == ConnectionTransfer {
		limboHost, limboPort, err := net.SplitHostPort(p.LimboUpstream)
		if err != nil || strings.TrimSpace(limboHost) == "" {
			return fmt.Errorf("Limbo 地址格式应为 IP:端口")
		}
		if port, err := strconv.Atoi(limboPort); err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("Limbo 端口无效")
		}
	}
	host, port, err := net.SplitHostPort(p.Upstream)
	if err != nil || strings.TrimSpace(host) == "" {
		return fmt.Errorf("服务器地址格式应为 IP:端口")
	}
	upstreamPort, err := strconv.Atoi(port)
	if err != nil || upstreamPort < 1 || upstreamPort > 65535 {
		return fmt.Errorf("服务器端口无效")
	}
	if p.MaxPlayers < 2 || p.MaxPlayers > 10 {
		return fmt.Errorf("房间总人数必须为 2–10（含房主机器人）")
	}
	if name := len([]rune(p.RoomName)); name < 1 || name > 40 {
		return fmt.Errorf("房间名称必须为 1–40 字")
	}
	if len(p.RoomPassword) > 64 {
		return fmt.Errorf("房间密码过长")
	}
	if len([]rune(p.LimboFormTitle)) > 64 {
		return fmt.Errorf("转服表单标题最多 64 字")
	}
	if len([]rune(p.LimboFormContent)) > 2000 {
		return fmt.Errorf("转服表单内容最多 2000 字")
	}
	if len([]rune(p.LimboFormButton)) > 64 {
		return fmt.Errorf("转服按钮文字最多 64 字")
	}
	version := strings.Split(p.RoomGameVersion, ".")
	if len(version) != 4 {
		return fmt.Errorf("网易房间内核版本应为四段数字，例如 1.21.130.0")
	}
	for _, part := range version {
		if _, err := strconv.ParseUint(part, 10, 16); err != nil {
			return fmt.Errorf("网易房间内核版本格式无效")
		}
	}
	if p.LobbyProtocolID == 0 {
		return fmt.Errorf("房间协议版本不能为 0")
	}
	if p.RetrySeconds < 5 || p.RetrySeconds > 300 {
		return fmt.Errorf("重连间隔必须为 5–300 秒")
	}
	return nil
}

// Validate checks installation-wide settings and cross-profile constraints.
func (c Config) Validate() error {
	if len(c.Profiles) == 0 {
		return fmt.Errorf("至少需要一个配置方案")
	}
	if len(c.Profiles) > maxProfiles {
		return fmt.Errorf("配置方案最多 %d 个", maxProfiles)
	}
	ids := make(map[string]bool, len(c.Profiles))
	names := make(map[string]bool, len(c.Profiles))
	type endpoint struct {
		host string
		port string
	}
	var limbos []endpoint
	for _, profile := range c.Profiles {
		if err := profile.Validate(); err != nil {
			return fmt.Errorf("%s：%w", profile.Name, err)
		}
		if ids[profile.ID] {
			return fmt.Errorf("配置方案 ID 重复：%s", profile.ID)
		}
		ids[profile.ID] = true
		name := strings.ToLower(strings.TrimSpace(profile.Name))
		if names[name] {
			return fmt.Errorf("配置方案名称重复：%s", profile.Name)
		}
		names[name] = true
		if profile.ConnectionMode != ConnectionTransfer {
			continue
		}
		host, port, err := net.SplitHostPort(profile.LimboUpstream)
		if err != nil {
			return fmt.Errorf("%s：Limbo 地址格式应为 IP:端口", profile.Name)
		}
		host = strings.ToLower(strings.TrimSpace(host))
		for _, existing := range limbos {
			if existing.port == port &&
				(existing.host == host || isWildcardHost(existing.host) || isWildcardHost(host)) {
				return fmt.Errorf("配置方案「%s」与另一个转服方案使用了相同的 Limbo 端口 %s", profile.Name, port)
			}
		}
		limbos = append(limbos, endpoint{host: host, port: port})
	}
	if c.WebPort < 1024 || c.WebPort > 65535 {
		return fmt.Errorf("控制台端口必须为 1024–65535")
	}
	return nil
}

func isWildcardHost(host string) bool {
	return host == "" || host == "0.0.0.0" || host == "::"
}

// Profile returns the profile with the exact ID.
func (c Config) Profile(id string) (Profile, bool) {
	for _, profile := range c.Profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return Profile{}, false
}

// DefaultProfileID returns the stable fallback profile: "default" when present,
// otherwise the first configured profile.
func (c Config) DefaultProfileID() string {
	if _, ok := c.Profile(DefaultProfileID); ok {
		return DefaultProfileID
	}
	if len(c.Profiles) > 0 {
		return c.Profiles[0].ID
	}
	return DefaultProfileID
}

// Clone deep-copies the configuration so callers cannot mutate manager state.
func (c Config) Clone() Config {
	c.Profiles = append([]Profile(nil), c.Profiles...)
	return c
}

// WithoutRoomPasswords returns a copy with every profile's room password
// cleared. It is used for status.json and default backups.
func (c Config) WithoutRoomPasswords() Config {
	next := c.Clone()
	for index := range next.Profiles {
		next.Profiles[index].RoomPassword = ""
	}
	return next
}

// legacyConfig mirrors the flat pre-profile configuration file.
type legacyConfig struct {
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

func legacyDefaults() legacyConfig {
	p := DefaultProfile()
	return legacyConfig{
		ConnectionMode:     p.ConnectionMode,
		LimboUpstream:      p.LimboUpstream,
		LimboAutoPullPacks: p.LimboAutoPullPacks,
		LimboFormTitle:     p.LimboFormTitle,
		LimboFormContent:   p.LimboFormContent,
		LimboFormButton:    p.LimboFormButton,
		LobbyProtocolID:    p.LobbyProtocolID,
		Upstream:           p.Upstream,
		RoomName:           p.RoomName,
		RoomGameVersion:    p.RoomGameVersion,
		MaxPlayers:         p.MaxPlayers,
		AutoAcceptFriends:  p.AutoAcceptFriends,
		AllowPvP:           p.AllowPvP,
		WebPort:            18321,
		RetrySeconds:       p.RetrySeconds,
	}
}

// LegacyPacksDir reproduces the pre-profile default resource pack directory:
// the explicit packs directory wins, then "packs" next to limbo_path or the
// running executable.
func LegacyPacksDir(limboPath string) string {
	base := ""
	if strings.TrimSpace(limboPath) != "" {
		base = filepath.Dir(limboPath)
	} else if executable, err := os.Executable(); err == nil {
		base = filepath.Dir(executable)
	}
	if base == "" {
		return ""
	}
	return filepath.Join(base, "packs")
}

func migrateLegacy(old legacyConfig) Config {
	profile := Profile{
		ID:                 DefaultProfileID,
		Name:               DefaultProfileName,
		ConnectionMode:     old.ConnectionMode,
		Upstream:           old.Upstream,
		LimboUpstream:      old.LimboUpstream,
		LimboPacksDir:      old.LimboPacksDir,
		LimboAutoPullPacks: old.LimboAutoPullPacks,
		LimboFormTitle:     old.LimboFormTitle,
		LimboFormContent:   old.LimboFormContent,
		LimboFormButton:    old.LimboFormButton,
		LobbyProtocolID:    old.LobbyProtocolID,
		RoomName:           old.RoomName,
		RoomGameVersion:    old.RoomGameVersion,
		RoomPassword:       old.RoomPassword,
		MaxPlayers:         old.MaxPlayers,
		FriendsOnly:        old.FriendsOnly,
		AutoAcceptFriends:  old.AutoAcceptFriends,
		AllowPvP:           old.AllowPvP,
		RetrySeconds:       old.RetrySeconds,
	}
	if strings.TrimSpace(profile.LimboPacksDir) != "" {
		if absolute, err := filepath.Abs(profile.LimboPacksDir); err == nil {
			profile.LimboPacksDir = absolute
		}
	} else {
		// Persist the old effective location so existing packs keep loading.
		profile.LimboPacksDir = LegacyPacksDir(old.LimboPath)
	}
	migrateProfile(&profile)
	cfg := Config{
		Profiles:  []Profile{profile},
		LimboPath: old.LimboPath,
		AutoStart: old.AutoStart,
		WebPort:   old.WebPort,
		Debug:     old.Debug,
	}
	cfg.Normalize()
	return cfg
}

// migrateProfile applies the version migrations that used to live in Load.
func migrateProfile(profile *Profile) {
	if profile.RoomGameVersion == "1.21.120.0" {
		profile.RoomGameVersion = "1.21.130.0"
	}
	if profile.LobbyProtocolID == 42 {
		profile.LobbyProtocolID = 43
	}
}

// Decode parses either the current profiles shape or a legacy flat config.
// legacy reports whether the input was migrated from the old shape.
func Decode(raw []byte) (cfg Config, legacy bool, err error) {
	var probe map[string]json.RawMessage
	if err = json.Unmarshal(raw, &probe); err != nil {
		return Config{}, false, err
	}
	if _, ok := probe["profiles"]; ok {
		cfg = Defaults()
		if err = json.Unmarshal(raw, &cfg); err != nil {
			return Config{}, false, err
		}
		for index := range cfg.Profiles {
			migrateProfile(&cfg.Profiles[index])
		}
		cfg.Normalize()
		return cfg, false, nil
	}
	old := legacyDefaults()
	if err = json.Unmarshal(raw, &old); err != nil {
		return Config{}, false, err
	}
	return migrateLegacy(old), true, nil
}

// Load reads config.json, creating it with defaults when absent. Unknown keys
// from older deployments are ignored and dropped on the next save.
func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Defaults(), Save(path, Defaults())
	}
	if err != nil {
		return Config{}, err
	}
	cfg, _, err := Decode(raw)
	if err != nil {
		return Config{}, err
	}
	if err = cfg.Validate(); err != nil {
		return cfg, err
	}
	normalized, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return cfg, err
	}
	normalized = append(normalized, '\n')
	if !bytes.Equal(bytes.TrimSpace(raw), bytes.TrimSpace(normalized)) {
		if err = fileutil.Replace(path, normalized, 0600); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

// Save validates and atomically writes the configuration.
func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.Replace(path, append(raw, '\n'), 0600)
}
