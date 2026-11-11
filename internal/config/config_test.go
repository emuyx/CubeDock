package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadCreatesDefaultsAndPersistsChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded, Defaults()) {
		t.Fatalf("defaults = %+v", loaded)
	}
	loaded.Profiles[0].RoomName = "CubeDock test"
	if err = Save(path, loaded); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil || again.Profiles[0].RoomName != "CubeDock test" {
		t.Fatalf("reload = %+v, %v", again, err)
	}
}

func TestLoadMigratesLegacyFlatConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	legacy := map[string]any{
		"auth_server": "https://legacy.example", "auth_mode": "embedded",
		"netease_engine_version": "3.9.15.297907", "max_players": 40,
		"lobby_protocol_id": 0, "upstream": "127.0.0.1:19132",
		"room_name": "test", "room_game_version": "1.21.120.0",
		"web_port": 18321, "retry_seconds": 10,
	}
	raw, _ := json.Marshal(legacy)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Profiles) != 1 {
		t.Fatalf("profiles = %+v", loaded.Profiles)
	}
	profile := loaded.Profiles[0]
	if profile.ID != DefaultProfileID || profile.Name != DefaultProfileName {
		t.Fatalf("migrated profile = %+v", profile)
	}
	if profile.MaxPlayers != 10 || profile.LobbyProtocolID != 43 ||
		profile.RoomGameVersion != "1.21.130.0" || profile.ConnectionMode != ConnectionProxy {
		t.Fatalf("legacy values not normalised: %+v", profile)
	}
	if profile.LimboPacksDir == "" {
		t.Fatal("legacy pack directory was not preserved")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, removed := range []string{"auth_server", "auth_mode", "netease_engine_version"} {
		if strings.Contains(string(after), removed) {
			t.Fatalf("removed field %q survived rewrite", removed)
		}
	}
	if !strings.Contains(string(after), `"profiles"`) {
		t.Fatalf("rewritten config is not profiles-shaped: %s", after)
	}
}

func TestLoadMigratesLegacyProfileValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	raw, _ := json.Marshal(map[string]any{
		"upstream": "127.0.0.1:19132", "room_name": "test",
		"lobby_protocol_id": 42, "room_game_version": "1.21.120.0",
		"web_port": 18321, "retry_seconds": 10,
	})
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	profile := loaded.Profiles[0]
	if profile.LobbyProtocolID != 43 || profile.RoomGameVersion != "1.21.130.0" {
		t.Fatalf("profile migration = %+v", profile)
	}
}

func TestLoadMigratesProfileValuesInNewShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	profile := DefaultProfile()
	profile.LobbyProtocolID = 42
	profile.RoomGameVersion = "1.21.120.0"
	raw, _ := json.Marshal(Config{Profiles: []Profile{profile}, WebPort: 18321})
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Profiles[0].LobbyProtocolID != 43 || loaded.Profiles[0].RoomGameVersion != "1.21.130.0" {
		t.Fatalf("profile migration = %+v", loaded.Profiles[0])
	}
}

func TestConnectionModes(t *testing.T) {
	cfg := Defaults()
	if cfg.Profiles[0].ConnectionMode != ConnectionProxy {
		t.Fatalf("default mode = %q", cfg.Profiles[0].ConnectionMode)
	}
	cfg.Profiles[0].ConnectionMode = ConnectionTransfer
	if err := cfg.Validate(); err != nil {
		t.Fatalf("transfer mode rejected: %v", err)
	}
	cfg.Profiles[0].LimboUpstream = "bad"
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid limbo address accepted")
	}
	cfg.Profiles[0].LimboUpstream = "127.0.0.1:19133"
	cfg.Profiles[0].ConnectionMode = "unknown"
	if err := cfg.Validate(); err == nil {
		t.Fatal("unknown connection mode accepted")
	}
	cfg.Profiles[0].ConnectionMode = ""
	cfg.Normalize()
	if cfg.Profiles[0].ConnectionMode != ConnectionProxy {
		t.Fatalf("empty mode normalized to %q", cfg.Profiles[0].ConnectionMode)
	}
}

func TestValidateRejectsUnsafeValues(t *testing.T) {
	for name, change := range map[string]func(*Profile){
		"connection mode": func(p *Profile) { p.ConnectionMode = "unknown" },
		"upstream":        func(p *Profile) { p.Upstream = "localhost" },
		"room name":       func(p *Profile) { p.RoomName = "" },
		"game version":    func(p *Profile) { p.RoomGameVersion = "1.21" },
		"max players":     func(p *Profile) { p.MaxPlayers = 1 },
		"retry interval":  func(p *Profile) { p.RetrySeconds = 1 },
		"form title":      func(p *Profile) { p.LimboFormTitle = strings.Repeat("x", 65) },
		"form content":    func(p *Profile) { p.LimboFormContent = strings.Repeat("x", 2001) },
		"form button":     func(p *Profile) { p.LimboFormButton = strings.Repeat("x", 65) },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := Defaults()
			change(&cfg.Profiles[0])
			if err := cfg.Validate(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
	cfg := Defaults()
	cfg.WebPort = 80
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid web port accepted")
	}
}

func TestValidateRejectsDuplicateProfiles(t *testing.T) {
	second := DefaultProfile()
	second.ID = "second"
	second.Name = "第二方案"
	second.Upstream = "127.0.0.1:19134"
	second.LimboUpstream = "127.0.0.1:19135"

	for name, change := range map[string]func(*Config){
		"id": func(c *Config) { c.Profiles[1].ID = c.Profiles[0].ID },
		"name": func(c *Config) { c.Profiles[1].Name = c.Profiles[0].Name },
		"limbo port": func(c *Config) {
			c.Profiles[0].ConnectionMode = ConnectionTransfer
			c.Profiles[1].ConnectionMode = ConnectionTransfer
			c.Profiles[1].LimboUpstream = "127.0.0.1:19133"
		},
		"wildcard overlap": func(c *Config) {
			c.Profiles[0].ConnectionMode = ConnectionTransfer
			c.Profiles[1].ConnectionMode = ConnectionTransfer
			c.Profiles[0].LimboUpstream = "0.0.0.0:19133"
			c.Profiles[1].LimboUpstream = "127.0.0.1:19133"
		},
		"empty": func(c *Config) { c.Profiles = nil },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := Defaults()
			cfg.Profiles = append(cfg.Profiles, second)
			change(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestCloneAndPasswordRedaction(t *testing.T) {
	cfg := Defaults()
	cfg.Profiles[0].RoomPassword = "secret"
	clone := cfg.Clone()
	clone.Profiles[0].RoomName = "changed"
	if cfg.Profiles[0].RoomName == "changed" {
		t.Fatal("Clone shares the profiles slice")
	}
	redacted := cfg.WithoutRoomPasswords()
	if redacted.Profiles[0].RoomPassword != "" {
		t.Fatal("room password was not redacted")
	}
	if cfg.Profiles[0].RoomPassword != "secret" {
		t.Fatal("redaction mutated the original")
	}
}

func TestDecodeNewShapeMarksNotLegacy(t *testing.T) {
	raw, _ := json.Marshal(Defaults())
	cfg, legacy, err := Decode(raw)
	if err != nil || legacy {
		t.Fatalf("Decode = %+v legacy=%v err=%v", cfg, legacy, err)
	}
	if len(cfg.Profiles) != 1 {
		t.Fatalf("profiles = %+v", cfg.Profiles)
	}
}
