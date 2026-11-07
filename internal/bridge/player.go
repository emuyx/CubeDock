package bridge

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/emuyx/cubedock/internal/gamebatch"
)

// maxInspectReads bounds how many early server batches are scanned for the
// PlayStatus that tells us whether the upstream accepted the player's login.
// LoginSuccess arrives before world data, so a small window is enough.
const maxInspectReads = 64

// PlayerInfo is the display identity carried by a local-room Login packet.
// UID is NetEase's account identifier; the console only prints it in debug mode.
type PlayerInfo struct {
	Name string
	UID  string
}

// SetPlayerHooks installs optional callbacks for player login metadata and the
// first upstream PlayStatus. Call it before relay.Pipe starts.
func (r *CryptoRelay) SetPlayerHooks(onLogin func(PlayerInfo), onPlayStatus func(PlayerInfo, int32)) {
	r.playerMu.Lock()
	r.onLogin = onLogin
	r.onPlayStatus = onPlayStatus
	r.playerMu.Unlock()
}

// Player returns the identity read from the client's Login packet, if any.
func (r *CryptoRelay) Player() PlayerInfo {
	r.playerMu.Lock()
	defer r.playerMu.Unlock()
	return r.player
}

// Joined reports whether the upstream sent PlayStatus LoginSuccess.
func (r *CryptoRelay) Joined() bool {
	r.playerMu.Lock()
	defer r.playerMu.Unlock()
	return r.joined
}

func (r *CryptoRelay) setPlayer(player PlayerInfo) {
	r.playerMu.Lock()
	r.player = player
	hook := r.onLogin
	r.playerMu.Unlock()
	if hook != nil {
		hook(player)
	}
}

// inspectServerPacket looks for a PlayStatus packet in an early server batch.
// It stops scanning once any PlayStatus has been seen, so normal traffic does
// not pay decompression cost.
func (r *CryptoRelay) inspectServerPacket(packet []byte) {
	r.playerMu.Lock()
	if r.statusSeen || r.inspectReads >= maxInspectReads {
		r.playerMu.Unlock()
		return
	}
	r.inspectReads++
	r.playerMu.Unlock()

	if len(packet) < 2 || packet[0] != 0xfe {
		return
	}
	body, err := gamebatch.Decode(packet[1:])
	if err != nil {
		return
	}
	packets, err := gamebatch.Split(body)
	if err != nil {
		return
	}
	for _, p := range packets {
		id, n := binary.Uvarint(p)
		if n <= 0 || id&0x3ff != 2 || len(p)-n < 4 {
			continue
		}
		r.notePlayStatus(int32(binary.BigEndian.Uint32(p[n:])))
		return
	}
}

func (r *CryptoRelay) notePlayStatus(status int32) {
	r.playerMu.Lock()
	if r.statusSeen {
		r.playerMu.Unlock()
		return
	}
	r.statusSeen = true
	if status == 0 {
		r.joined = true
	}
	player := r.player
	hook := r.onPlayStatus
	r.playerMu.Unlock()
	if hook != nil {
		hook(player, status)
	}
}

func playerInfoFromIdentity(identity map[string]any) PlayerInfo {
	player := PlayerInfo{
		Name: sanitizeDisplayName(claimText(identity, "displayName")),
		UID:  claimText(identity, "netease_uid"),
	}
	if player.UID == "" {
		player.UID = claimText(identity, "XUID")
	}
	if len(player.UID) > 64 {
		player.UID = player.UID[:64]
	}
	return player
}

func claimText(identity map[string]any, key string) string {
	switch value := identity[key].(type) {
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return value.String()
	case float64:
		return fmt.Sprintf("%.0f", value)
	default:
		return ""
	}
}

func sanitizeDisplayName(name string) string {
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
	return name
}

// PlayStatusName returns a credential-free label for a Bedrock PlayStatus.
func PlayStatusName(status int32) string {
	switch status {
	case 0:
		return "登录成功"
	case 1:
		return "客户端登录失败"
	case 2:
		return "服务器登录失败"
	case 3:
		return "玩家生成完成"
	case 4:
		return "租赁服务器验证失败"
	case 5:
		return "教育版与正式版不兼容"
	case 6:
		return "正式版与教育版不兼容"
	case 7:
		return "服务器已满"
	case 8:
		return "编辑器与正式版不兼容"
	case 9:
		return "正式版与编辑器不兼容"
	default:
		return fmt.Sprintf("未知状态 %d", status)
	}
}
