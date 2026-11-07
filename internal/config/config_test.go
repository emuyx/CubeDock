package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCreatesDefaultsAndPersistsChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != Defaults() {
		t.Fatalf("defaults = %+v", loaded)
	}
	loaded.RoomName = "CubeDock test"
	if err = Save(path, loaded); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil || again.RoomName != "CubeDock test" {
		t.Fatalf("reload = %+v, %v", again, err)
	}
}

func TestLoadDropsLegacyAuthAndVersionKeys(t *testing.T) {
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
	if loaded.MaxPlayers != 10 || loaded.LobbyProtocolID != 43 ||
		loaded.RoomGameVersion != "1.21.130.0" || loaded.ConnectionMode != ConnectionProxy {
		t.Fatalf("legacy values not normalised: %+v", loaded)
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
}

func TestLoadMigratesLegacyLobbyProtocol(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Defaults()
	cfg.LobbyProtocolID = 42
	raw, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LobbyProtocolID != 43 {
		t.Fatalf("lobby protocol = %d, want 43", loaded.LobbyProtocolID)
	}
}

func TestConnectionModes(t *testing.T) {
	cfg := Defaults()
	if cfg.ConnectionMode != ConnectionProxy {
		t.Fatalf("default mode = %q", cfg.ConnectionMode)
	}
	cfg.ConnectionMode = ConnectionTransfer
	if err := cfg.Validate(); err != nil {
		t.Fatalf("transfer mode rejected: %v", err)
	}
	cfg.LimboUpstream = "bad"
	if err := cfg.Validate(); err == nil {
		t.Fatal("invalid limbo address accepted")
	}
	cfg.LimboUpstream = "127.0.0.1:19133"
	cfg.ConnectionMode = "unknown"
	if err := cfg.Validate(); err == nil {
		t.Fatal("unknown connection mode accepted")
	}
	cfg.ConnectionMode = ""
	cfg.Normalize()
	if cfg.ConnectionMode != ConnectionProxy {
		t.Fatalf("empty mode normalized to %q", cfg.ConnectionMode)
	}
}

func TestValidateRejectsUnsafeValues(t *testing.T) {
	for name, change := range map[string]func(*Config){
		"connection mode": func(c *Config) { c.ConnectionMode = "unknown" },
		"upstream":        func(c *Config) { c.Upstream = "localhost" },
		"room name":       func(c *Config) { c.RoomName = "" },
		"game version":    func(c *Config) { c.RoomGameVersion = "1.21" },
		"max players":     func(c *Config) { c.MaxPlayers = 1 },
		"web port":        func(c *Config) { c.WebPort = 80 },
		"retry interval":  func(c *Config) { c.RetrySeconds = 1 },
		"form title":      func(c *Config) { c.LimboFormTitle = strings.Repeat("x", 65) },
		"form content":    func(c *Config) { c.LimboFormContent = strings.Repeat("x", 2001) },
		"form button":     func(c *Config) { c.LimboFormButton = strings.Repeat("x", 65) },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := Defaults()
			change(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
