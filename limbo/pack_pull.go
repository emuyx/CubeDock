package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/df-mc/jsonc"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

var errPacksPulled = errors.New("resource packs pulled")

// safeLog strips control characters from remote-provided values before they
// are written to the log, so a pack version cannot forge extra log lines.
func safeLog(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' || r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
}

// pullResourcePacks logs into the backend server as a bot and downloads all
// unencrypted resource packs into dir. It prints a JSON summary on success.
func pullResourcePacks(upstream, dir, name, uid, network string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return pullResourcePacksContext(ctx, upstream, dir, name, uid, network)
}

func pullResourcePacksContext(ctx context.Context, upstream, dir, name, uid, network string) error {
	upstream = strings.TrimSpace(upstream)
	if upstream == "" {
		return errors.New("拉取资源包需要 -target 后端服地址")
	}
	if network == "" {
		network = "netease-v8"
	}
	dir, err := ensurePullPacksDir(dir)
	if err != nil {
		return err
	}
	name = uniqueBotName(botDisplayName(name))
	uid = strings.TrimSpace(uid)
	if uid == "" {
		uid = "1000000000"
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	local := localPackKeys(dir)
	skippedLocal := 0
	saved, skipped := 0, 0
	var saveErr error
	dialer := minecraft.Dialer{
		IdentityData: login.IdentityData{
			Identity:    uuid.NewString(),
			DisplayName: name,
			NetEaseUID:  uid,
			NetEaseUUID: uuid.NewString(),
		},
		DownloadResourcePack: func(id uuid.UUID, version string, current, total int) bool {
			if _, ok := local[packKey(id.String(), version)]; ok {
				log.Printf("[packs] 本地已有 %s v%s（%d/%d），跳过下载", id, safeLog(version), current+1, total)
				skippedLocal++
				return false
			}
			log.Printf("[packs] 下载后端资源包 %s v%s（%d/%d）", id, safeLog(version), current+1, total)
			return true
		},
		OnResourcePacksComplete: func(conn *minecraft.Conn) error {
			// Packs are downloaded and the server sent the stack. Disconnect
			// before sending Completed/entering the world.
			log.Printf("[packs] 资源包下载完成，中止进入服务器")
			sendPullDisconnect()
			saved, skipped, saveErr = savePulledPacks(conn.ResourcePacks(), dir, skippedLocal)
			if saveErr != nil {
				return saveErr
			}
			return errPacksPulled
		},
		DisconnectOnUnknownPackets: false,
		DisconnectOnInvalidPackets: false,
		ErrorLog:                   logger,
	}
	log.Printf("[packs] 以 %s（UID %s）通过 %s 登录后端服 %s 拉取资源包", name, uid, network, upstream)
	conn, err := dialer.DialContext(ctx, network, upstream)
	if err != nil && !errors.Is(err, errPacksPulled) {
		return fmt.Errorf("登录后端服失败: %w", err)
	}
	if conn != nil {
		// Fallback when the server never sent a stack; the hook did not run.
		sendPullDisconnect()
		saved, skipped, saveErr = savePulledPacks(conn.ResourcePacks(), dir, skippedLocal)
		_ = conn.Close()
	}
	if saveErr != nil {
		return saveErr
	}
	switch {
	case saved == 0 && skippedLocal > 0:
		log.Printf("[packs] %d 个后端资源包本地均已存在，未触发下载", skippedLocal)
	case saved == 0 && skipped == 0:
		log.Printf("[packs] 后端服没有资源包")
	}
	summary, _ := json.Marshal(map[string]any{
		"dir":           dir,
		"pulled":        saved,
		"skipped":       skipped,
		"skipped_local": skippedLocal,
		"total":         saved + skipped,
	})
	fmt.Println(string(summary))
	return nil
}

const (
	// maxPulledPackSize bounds one pack and maxPulledPackTotal bounds a single
	// pull. Both come from a remote server; without a cap they can fill the
	// disk or exhaust memory.
	maxPulledPackSize  = 512 << 20
	maxPulledPackTotal = 2 << 30
)

// savePulledPacks writes downloaded packs to dir. skipped is the number of
// packs already present locally; it is incremented for encrypted or unreadable
// packs.
func savePulledPacks(packs []*resource.Pack, dir string, skipped int) (saved, skippedTotal int, err error) {
	total := int64(0)
	for _, pack := range packs {
		if pack.Encrypted() {
			log.Printf("[packs] 跳过加密资源包 %s v%s（当前 limbo 不支持 ContentKey）", pack.UUID(), safeLog(pack.Version()))
			skipped++
			continue
		}
		size := pack.Len()
		if size < 0 || size > maxPulledPackSize {
			log.Printf("[packs] 跳过超过上限的资源包 %s v%s（%d 字节）", pack.UUID(), safeLog(pack.Version()), size)
			skipped++
			continue
		}
		if total+int64(size) > maxPulledPackTotal {
			return saved, skipped, fmt.Errorf("资源包总量超过上限 %d 字节", maxPulledPackTotal)
		}
		data := make([]byte, size)
		if _, err := pack.ReadAt(data, 0); err != nil && !errors.Is(err, io.EOF) {
			log.Printf("[packs] 读取资源包 %s v%s 失败: %v", pack.UUID(), safeLog(pack.Version()), err)
			skipped++
			continue
		}
		target := filepath.Join(dir, pack.UUID().String()+"_"+pack.Version()+".mcpack")
		if existing, err := os.ReadFile(target); err == nil && bytes.Equal(existing, data) {
			log.Printf("[packs] 已存在同名资源包 %s，跳过写入", filepath.Base(target))
			continue
		}
		tmp, err := os.CreateTemp(dir, ".pack-*.tmp")
		if err != nil {
			return 0, skipped, fmt.Errorf("创建资源包临时文件失败: %w", err)
		}
		tmpName := tmp.Name()
		if _, err = tmp.Write(data); err == nil {
			err = tmp.Close()
		} else {
			_ = tmp.Close()
		}
		if err != nil {
			_ = os.Remove(tmpName)
			return 0, skipped, fmt.Errorf("写入资源包失败: %w", err)
		}
		_ = os.Remove(target)
		if err := os.Rename(tmpName, target); err != nil {
			_ = os.Remove(tmpName)
			return 0, skipped, fmt.Errorf("保存资源包失败: %w", err)
		}
		log.Printf("[packs] 已保存 %s（%d 字节）", filepath.Base(target), len(data))
		saved++
		total += int64(len(data))
	}
	return saved, skipped, nil
}

// sendPullDisconnect writes a RakNet DisconnectNotification (0x15) directly on
// the NetEase v8 connection. This lets Geyser/Java login plugins release the
// bot before it spawns. Standard RakNet tests have no raw conn and rely on the
// client closing the connection.
func sendPullDisconnect() {
	raw := takePullConn()
	if raw == nil {
		return
	}
	if _, err := raw.Write([]byte{0x15}); err != nil {
		log.Printf("[packs] 发送 RakNet 断开包失败: %v", err)
		return
	}
	log.Printf("[packs] 已向服务器发送 RakNet 断开包")
	time.Sleep(300 * time.Millisecond)
}

func ensurePullPacksDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", fmt.Errorf("解析资源包目录失败: %w", err)
		}
		if err := os.MkdirAll(abs, 0755); err != nil {
			return "", fmt.Errorf("创建资源包目录失败: %w", err)
		}
		return abs, nil
	}
	if resolved, ok := findPacksDir(); ok {
		return resolved, nil
	}
	return "", errors.New("无法确定资源包目录")
}

type pullManifest struct {
	Header struct {
		UUID    string `json:"uuid"`
		Version []int  `json:"version"`
	} `json:"header"`
}

// localPackKeys indexes existing packs by lower-case UUID and manifest version.
// A server pack with the same UUID+version is not downloaded again.
func localPackKeys(dir string) map[string]struct{} {
	keys := make(map[string]struct{})
	entries, err := os.ReadDir(dir)
	if err != nil {
		return keys
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		var key string
		var ok bool
		if entry.IsDir() {
			key, ok = directoryPackKey(path)
		} else {
			key, ok = zipPackKey(path)
		}
		if ok {
			keys[key] = struct{}{}
		}
	}
	return keys
}

func packKey(id, version string) string {
	return strings.ToLower(strings.TrimSpace(id)) + "_" + strings.TrimSpace(version)
}

func manifestPackKey(data []byte) (string, bool) {
	var manifest pullManifest
	if jsonc.Unmarshal(data, &manifest) != nil || manifest.Header.UUID == "" || len(manifest.Header.Version) < 3 {
		return "", false
	}
	version := fmt.Sprintf("%d.%d.%d",
		manifest.Header.Version[0], manifest.Header.Version[1], manifest.Header.Version[2])
	return packKey(manifest.Header.UUID, version), true
}

func zipPackKey(path string) (string, bool) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", false
	}
	defer reader.Close()
	for _, file := range reader.File {
		if !strings.EqualFold(filepath.Base(file.Name), "manifest.json") {
			continue
		}
		read, err := file.Open()
		if err != nil {
			return "", false
		}
		data, err := io.ReadAll(io.LimitReader(read, 1<<20))
		_ = read.Close()
		if err != nil {
			return "", false
		}
		return manifestPackKey(data)
	}
	return "", false
}

func directoryPackKey(path string) (string, bool) {
	var manifest []byte
	found := false
	_ = filepath.WalkDir(path, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() && !found && strings.EqualFold(entry.Name(), "manifest.json") {
			if data, readErr := os.ReadFile(current); readErr == nil {
				manifest = data
				found = true
			}
		}
		return nil
	})
	if !found {
		return "", false
	}
	return manifestPackKey(manifest)
}

// uniqueBotName keeps repeated pulls from colliding with the server's
// single-session-per-name check. The suffix is short but unique per pull.
func uniqueBotName(name string) string {
	suffix := "-" + strings.ToUpper(uuid.NewString()[:4])
	runes := []rune(name)
	limit := 32 - len(suffix)
	if limit < 1 {
		limit = 1
	}
	if len(runes) > limit {
		name = string(runes[:limit])
	}
	return name + suffix
}

func botDisplayName(name string) string {
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name))
	runes := []rune(name)
	if len(runes) > 32 {
		name = string(runes[:32])
	}
	if name == "" {
		name = "CubeDockPackBot"
	}
	return name
}
