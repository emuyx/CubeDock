package main

import (
	"archive/zip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const testPackManifest = `{
  "format_version": 2,
  "header": {
    "description": "CubeDock limbo test pack",
    "name": "CubeDock limbo test pack",
    "uuid": "11111111-2222-3333-4444-555555555555",
    "version": [1, 0, 0],
    "min_engine_version": [1, 21, 0]
  },
  "modules": [
    {
      "type": "resources",
      "uuid": "66666666-7777-8888-9999-000000000000",
      "version": [1, 0, 0]
    }
  ]
}`

func TestLoadResourcePacksExplicit(t *testing.T) {
	dir := t.TempDir()
	if err := writePackZip(filepath.Join(dir, "test.mcpack"), testPackManifest); err != nil {
		t.Fatal(err)
	}
	packDir := filepath.Join(dir, "folder-pack")
	if err := os.Mkdir(packDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "manifest.json"), []byte(testPackManifest), 0600); err != nil {
		t.Fatal(err)
	}

	packs, resolved, err := loadResourcePacks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != dir {
		t.Fatalf("resolved = %q, want %q", resolved, dir)
	}
	if len(packs) != 2 {
		t.Fatalf("packs = %d, want 2", len(packs))
	}
	for _, pack := range packs {
		if pack.UUID().String() != "11111111-2222-3333-4444-555555555555" {
			t.Fatalf("uuid = %s", pack.UUID())
		}
		if pack.Version() != "1.0.0" {
			t.Fatalf("version = %s", pack.Version())
		}
	}
}

func TestLoadResourcePacksMissingDir(t *testing.T) {
	if _, _, err := loadResourcePacks(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing pack directory accepted")
	}
}

func TestLoadResourcePacksEmptyDir(t *testing.T) {
	if _, _, err := loadResourcePacks(t.TempDir()); err == nil {
		t.Fatal("empty pack directory accepted")
	}
}

func TestPullResourcePacksFromStandardServer(t *testing.T) {
	src := t.TempDir()
	if err := writePackZip(filepath.Join(src, "test.mcpack"), testPackManifest); err != nil {
		t.Fatal(err)
	}
	packs, _, err := loadResourcePacks(src)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := (&minecraft.ListenConfig{
		AuthenticationDisabled: true,
		AllowUnknownPackets:    true,
		AllowInvalidPackets:    true,
		ResourcePacks:          packs,
		TexturePacksRequired:   true,
		ErrorLog:               slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
		PacketFunc: func(h packet.Header, payload []byte, src, dst net.Addr) {
			t.Logf("pkt %d len=%d src=%v dst=%v", h.PacketID, len(payload), src, dst)
		},
	}).Listen("raknet", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	form, err := buildSimpleForm("test", "test", "test")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		accepted, err := listener.Accept()
		if err != nil {
			return
		}
		serve(accepted.(*minecraft.Conn), "", 0, true, 0, form)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dst := t.TempDir()
	if err := pullResourcePacksContext(ctx, listener.Addr().String(), dst, "CubeDockTestBot", "1000000001", "raknet"); err != nil {
		t.Fatal(err)
	}
	pulled, _, err := loadResourcePacks(dst)
	if err != nil {
		t.Fatal(err)
	}
	if len(pulled) != 1 || pulled[0].UUID().String() != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("pulled packs = %d", len(pulled))
	}
}

func TestParseLenientIgnoresInvalidClientData(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	identity := login.IdentityData{
		Identity:    uuid.NewString(),
		DisplayName: "张哥乏足球梦想",
	}
	client := login.ClientData{
		DeviceOS:          7,
		GameVersion:       "1.21.130",
		LanguageCode:      "zh_CN",
		SelfSignedID:      uuid.NewString(),
		ServerAddress:     "127.0.0.1:19132",
		SkinID:            "test",
		SkinImageWidth:    1,
		SkinImageHeight:   1,
		SkinData:          "AAAA", // 3 bytes, invalid for 1x1 RGBA
		SkinResourcePatch: "e30=",
	}
	request := login.EncodeOffline(identity, client, key, "", false)
	if _, _, _, err := login.Parse(request, nil); err == nil {
		t.Fatal("strict parse accepted invalid skin data")
	}
	_, _, result, err := login.ParseLenient(request, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PublicKey == nil || !result.PublicKey.Equal(&key.PublicKey) {
		t.Fatal("lenient parse did not return the client public key")
	}
}

func TestLocalPackKeysAcceptsJSONC(t *testing.T) {
	dir := t.TempDir()
	if err := writePackZip(filepath.Join(dir, "commented.mcpack"), "// comment\n"+testPackManifest); err != nil {
		t.Fatal(err)
	}
	keys := localPackKeys(dir)
	key := packKey("11111111-2222-3333-4444-555555555555", "1.0.0")
	if _, ok := keys[key]; !ok {
		t.Fatalf("local keys = %v", keys)
	}
}

func TestBuildSimpleForm(t *testing.T) {
	data, err := buildSimpleForm(" 测试标题 ", "第一行\n第二行", "")
	if err != nil {
		t.Fatal(err)
	}
	var form struct {
		Type    string `json:"type"`
		Title   string `json:"title"`
		Content string `json:"content"`
		Buttons []struct {
			Text string `json:"text"`
		} `json:"buttons"`
	}
	if err := json.Unmarshal(data, &form); err != nil {
		t.Fatal(err)
	}
	if form.Type != "form" || form.Title != "测试标题" ||
		form.Content != "第一行\n第二行" ||
		len(form.Buttons) != 2 || form.Buttons[0].Text != "进入服务器" ||
		form.Buttons[1].Text != "断开连接" {
		t.Fatalf("form = %+v", form)
	}
}

func writePackZip(path, manifest string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("manifest.json")
	if err != nil {
		_ = file.Close()
		return err
	}
	if _, err = entry.Write([]byte(manifest)); err != nil {
		_ = file.Close()
		return err
	}
	if err = writer.Close(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
