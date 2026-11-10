package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

const (
	spawnY        = 65.0
	bedrockY      = 64
	bedrockRuntim = 13064 // Geyser 1.21.120/1.21.130 block palette index.
)

func main() {
	listen := flag.String("listen", "127.0.0.1:19133", "Bedrock limbo listen address")
	target := flag.String("target", "", "transfer target host:port")
	watchParent := flag.Bool("watch-parent", true, "exit when stdin closes")
	packsDir := flag.String("packs", "", "resource pack directory; empty searches <limbo.exe>/packs then ./packs")
	packsRequired := flag.Bool("packs-required", false, "mark loaded resource packs as required in ResourcePacksInfo")
	packsMode := flag.String("packs-mode", "netease", "resource pack handshake mode: netease (force download) or standard")
	packsSettle := flag.Duration("packs-settle", 3*time.Second, "wait after spawn before showing the form when packs are loaded")
	formTitle := flag.String("form-title", "CubeDock Limbo", "SimpleForm title")
	formContent := flag.String("form-content", "点击下方按钮进入服务器。", "SimpleForm content")
	formButton := flag.String("form-button", "进入服务器", "SimpleForm button text")
	pull := flag.Bool("pull", false, "download resource packs from -target and exit")
	pullNetwork := flag.String("pull-network", "netease-v8", "pull transport: netease-v8 or raknet (local testing)")
	identityName := flag.String("identity-name", "CubeDockPackBot", "bot display name used for the pack pull login")
	identityUID := flag.String("identity-uid", "1000000000", "bot NetEase UID used for the pack pull login")
	flag.Parse()

	if *pull {
		if err := pullResourcePacks(*target, *packsDir, *identityName, *identityUID, *pullNetwork); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *watchParent {
		go func() {
			_, _ = os.Stdin.Read(make([]byte, 1))
			os.Exit(0)
		}()
	}
	host, port, err := splitTarget(*target)
	if err != nil {
		log.Fatal(err)
	}
	formData, err := buildSimpleForm(*formTitle, *formContent, *formButton)
	if err != nil {
		log.Fatal(err)
	}
	packs, resolvedDir, err := loadResourcePacks(*packsDir)
	if err != nil {
		log.Fatal(err)
	}
	if resolvedDir != "" {
		encrypted := 0
		for _, pack := range packs {
			if pack.Encrypted() {
				encrypted++
			}
			log.Printf("resource pack: %s v%s (%d bytes)", pack.UUID(), pack.Version(), pack.Len())
		}
		log.Printf("loaded %d resource pack(s) from %s (%d encrypted)", len(packs), resolvedDir, encrypted)
	} else {
		log.Printf("no resource pack directory found; starting without resource packs")
	}

	var packChunkSeen atomic.Bool
	cfg := minecraft.ListenConfig{
		AuthenticationDisabled: true,
		// Bound the pre-login and idle connections limbo will hold. CubeDock
		// starts one limbo for all rooms, so the limit must cover several rooms.
		MaximumPlayers:            256,
		AllowUnknownPackets:       true,
		AllowInvalidPackets:       true,
		ResourcePacks:             packs,
		TexturePacksRequired:      *packsRequired && len(packs) > 0,
		ResourcePackForceDownload: *packsMode != "standard",
		StatusProvider:            minecraft.NewStatusProvider("CubeDock Limbo", "CubeDock"),
		ErrorLog:                  slog.New(slog.NewTextHandler(os.Stderr, nil)),
		PacketFunc: func(h packet.Header, payload []byte, src, dst net.Addr) {
			switch h.PacketID {
			case packet.IDResourcePacksInfo:
				log.Printf("[packs] sent ResourcePacksInfo")
			case packet.IDResourcePackClientResponse:
				if len(payload) > 0 {
					log.Printf("[packs] client response=%d", payload[0])
				}
			case packet.IDResourcePackDataInfo:
				log.Printf("[packs] sent ResourcePackDataInfo")
			case packet.IDResourcePackChunkRequest:
				if packChunkSeen.CompareAndSwap(false, true) {
					log.Printf("[packs] client started downloading")
				}
			case packet.IDResourcePackStack:
				log.Printf("[packs] sent ResourcePackStack")
			}
		},
	}
	listener, err := cfg.Listen("raknet", *listen)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	log.Printf("limbo listening on %s; target=%s; packs-mode=%s", *listen,
		net.JoinHostPort(host, strconv.Itoa(int(port))), *packsMode)

	for {
		c, err := listener.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			return
		}
		conn, ok := c.(*minecraft.Conn)
		if !ok {
			_ = c.Close()
			continue
		}
		go serve(conn, host, port, len(packs) > 0, *packsSettle, formData)
	}
}

// loadResourcePacks reads every pack in dir. An empty dir searches the limbo
// executable directory first, then the working directory, so the launcher's
// bin/packs layout works without extra configuration. The returned path is
// empty when no packs directory was found.
func loadResourcePacks(dir string) ([]*resource.Pack, string, error) {
	dir = strings.TrimSpace(dir)
	explicit := dir != ""
	if explicit {
		resolved, err := filepath.Abs(dir)
		if err != nil {
			return nil, "", fmt.Errorf("解析资源包目录失败: %w", err)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, "", fmt.Errorf("资源包目录 %s 不可用: %w", resolved, err)
		}
		if !info.IsDir() {
			return nil, "", fmt.Errorf("资源包路径 %s 不是目录", resolved)
		}
		dir = resolved
	} else {
		resolved, found := findPacksDir()
		if !found {
			return nil, "", nil
		}
		dir = resolved
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, "", fmt.Errorf("读取资源包目录失败: %w", err)
	}
	packs := make([]*resource.Pack, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		pack, err := resource.ReadPath(path)
		if err != nil {
			return nil, "", fmt.Errorf("读取资源包 %s 失败: %w", name, err)
		}
		packs = append(packs, pack)
	}
	if explicit && len(packs) == 0 {
		return nil, "", fmt.Errorf("资源包目录 %s 中没有资源包", dir)
	}
	return packs, dir, nil
}

func findPacksDir() (string, bool) {
	candidates := make([]string, 0, 2)
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "packs"))
	}
	candidates = append(candidates, "packs")
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, true
		}
	}
	// Create the default directory so users have a place to drop packs.
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "packs")
		if err := os.MkdirAll(dir, 0755); err == nil {
			return dir, true
		}
	}
	return "", false
}

func splitTarget(target string) (string, uint16, error) {
	if target == "" {
		return "", 0, nil
	}
	host, portText, err := net.SplitHostPort(target)
	if err != nil || host == "" {
		return "", 0, fmt.Errorf("target must be host:port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", 0, fmt.Errorf("invalid target port")
	}
	return host, uint16(port), nil
}

// buildSimpleForm builds a Bedrock SimpleForm (ModalFormRequest type "form").
func buildSimpleForm(title, content, button string) ([]byte, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "CubeDock Limbo"
	}
	content = strings.TrimSpace(content)
	if content == "" {
		content = "点击下方按钮进入服务器。"
	}
	button = strings.TrimSpace(button)
	if button == "" {
		button = "进入服务器"
	}
	return json.Marshal(map[string]any{
		"type":    "form",
		"title":   title,
		"content": content,
		"buttons": []map[string]string{
			{"text": button},
			{"text": "断开连接"},
		},
	})
}

func serve(conn *minecraft.Conn, host string, port uint16, hasPacks bool, packSettle time.Duration, formData []byte) {
	defer conn.Close()
	data := minecraft.GameData{
		WorldName:                    "CubeDock Limbo",
		WorldSeed:                    0,
		Difficulty:                   0,
		EntityUniqueID:               1,
		EntityRuntimeID:              1,
		PlayerGameMode:               1,
		PlayerPosition:               mgl32.Vec3{0.5, spawnY, 0.5},
		Dimension:                    0,
		WorldSpawn:                   protocol.BlockPos{0, bedrockY, 0},
		WorldGameMode:                1,
		BaseGameVersion:              "1.21.130",
		ChunkRadius:                  1,
		PlayerPermissions:            1,
		ServerAuthoritativeInventory: true,
	}
	if err := conn.StartGameTimeout(data, time.Minute); err != nil {
		log.Printf("start game: %v", err)
		return
	}
	if hasPacks {
		log.Printf("[packs] resource pack stage completed for %s", conn.RemoteAddr())
	}
	if err := sendEmptyChunk(conn); err != nil {
		log.Printf("empty chunk: %v", err)
		return
	}
	if err := conn.WritePacket(&packet.NetworkChunkPublisherUpdate{
		Position: protocol.BlockPos{0, bedrockY, 0},
		Radius:   16,
	}); err != nil {
		log.Printf("chunk publisher: %v", err)
		return
	}
	if err := conn.WritePacket(&packet.UpdateBlock{
		Position:          protocol.BlockPos{0, bedrockY, 0},
		NewBlockRuntimeID: bedrockRuntim,
		Flags:             packet.BlockUpdateNetwork,
		Layer:             0,
	}); err != nil {
		log.Printf("bedrock block: %v", err)
		return
	}
	if hasPacks && packSettle > 0 {
		log.Printf("resource packs accepted by %s; waiting %s before showing the form", conn.RemoteAddr(), packSettle)
		time.Sleep(packSettle)
	}

	formID := uint32(1)
	sendForm := func() error {
		return conn.WritePacket(&packet.ModalFormRequest{FormID: formID, FormData: formData})
	}
	if err := sendForm(); err != nil {
		log.Printf("send form: %v", err)
		return
	}

	for {
		pk, err := conn.ReadPacket()
		if err != nil {
			return
		}
		response, ok := pk.(*packet.ModalFormResponse)
		if !ok || response.FormID != formID {
			continue
		}
		data, ok := response.ResponseData.Value()
		if !ok {
			// Closing the form is treated as accidental; reopen it after a
			// short pause so the player can still choose an action.
			log.Printf("form closed without selection; reopening")
			time.Sleep(time.Second)
			formID++
			if err := sendForm(); err != nil {
				log.Printf("resend form: %v", err)
				return
			}
			continue
		}
		var selected int
		if json.Unmarshal(data, &selected) != nil {
			log.Printf("form selection invalid: %s; reopening", string(data))
			time.Sleep(time.Second)
			formID++
			if err := sendForm(); err != nil {
				log.Printf("resend form: %v", err)
				return
			}
			continue
		}
		switch selected {
		case 0:
			if host == "" {
				log.Printf("form accepted; no transfer target configured")
				return
			}
			if err := conn.WritePacket(&packet.Transfer{
				Address:     host,
				Port:        port,
				ReloadWorld: false,
			}); err != nil {
				log.Printf("send transfer: %v", err)
				return
			}
			log.Printf("form accepted; transferred to %s", net.JoinHostPort(host, strconv.Itoa(int(port))))
			return
		case 1:
			_ = conn.WritePacket(&packet.Disconnect{
				Reason:                  packet.DisconnectReasonUserLeaveGameAttempted,
				HideDisconnectionScreen: true,
				Message:                 "已断开连接",
			})
			_ = conn.Flush()
			log.Printf("player chose disconnect from limbo form")
			time.Sleep(200 * time.Millisecond)
			return
		default:
			log.Printf("unknown form selection %d; reopening", selected)
			time.Sleep(time.Second)
			formID++
			if err := sendForm(); err != nil {
				log.Printf("resend form: %v", err)
				return
			}
		}
	}
}

func sendEmptyChunk(conn *minecraft.Conn) error {
	// Geyser's empty chunk payload: empty biome storage + empty subchunk markers.
	const subChunks = 24
	payload := []byte{0x01, 0x00}
	for i := 0; i < subChunks-1; i++ {
		payload = append(payload, 0xff)
	}
	payload = append(payload, 0x00)
	return conn.WritePacket(&packet.LevelChunk{
		Position:      protocol.ChunkPos{0, 0},
		Dimension:     0,
		SubChunkCount: 0,
		CacheEnabled:  false,
		RawPayload:    payload,
	})
}
