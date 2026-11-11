package room

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"time"

	"github.com/Happy2018new/nemc-tan-lobby-solver/core/nethernet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/core/raknet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/core/webrtc"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/encoding"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/packet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/service/signaling"

	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/config"
)

// Open authenticates against the transfer node, creates the room and starts
// the signaling session. trace may be nil.
func Open(ctx context.Context, profile config.Profile, info auth.RoomInfo, trace func(string)) (host *Host, err error) {
	created, err := newHost(trace)
	if err != nil {
		return nil, err
	}
	host = created
	opened := false
	defer func() {
		if !opened {
			host.Close()
		}
	}()

	stopCancel, err := host.connectControl(ctx, info)
	if err != nil {
		return nil, err
	}
	defer stopCancel()

	if err = host.authenticateControl(info); err != nil {
		return nil, err
	}
	if err = host.createRoom(profile, info); err != nil {
		return nil, err
	}
	if err = host.openSignaling(ctx, info); err != nil {
		return nil, err
	}
	if err = host.startListener(ctx, trace); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	opened = true
	host.ready = newReadyQueue(host.done, host.notifyReady, trace)
	go host.controlLoop()
	return host, nil
}

func newHost(trace func(string)) (*Host, error) {
	host := &Host{
		trace: trace, done: make(chan struct{}),
		guestPlayers: make(map[uint64]uint32),
		members:      make(map[uint32]*RoomMember),
	}
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, err
	}
	host.networkID = binary.LittleEndian.Uint64(raw[:])
	if host.networkID == 0 {
		host.networkID = 1
	}
	return host, nil
}

func (h *Host) connectControl(ctx context.Context, info auth.RoomInfo) (func(), error) {
	dialCtx, stopDial := context.WithTimeout(ctx, 10*time.Second)
	conn, err := raknet.DialContext(dialCtx, info.RaknetAddress)
	stopDial()
	if err != nil {
		return nil, fmt.Errorf("房间转发节点连接失败: %w", err)
	}
	h.control = conn
	// Closing the control connection is what cancels registrations, not just
	// the initial UDP handshake.
	stop := context.AfterFunc(ctx, func() { _ = h.control.Close() })
	return func() { stop() }, nil
}

func (h *Host) authenticateControl(info auth.RoomInfo) error {
	h.encoder = packet.NewEncoder(h.control)
	decoder, err := packet.NewDecoder(h.control)
	if err != nil {
		return err
	}
	h.decoder = decoder

	if err := h.send(&packet.TanLoginRequest{
		PlayerID: info.UserID, Rand: info.RaknetRand, AESRand: info.RaknetAESRand,
		PlayerName: info.PlayerName,
	}); err != nil {
		return err
	}
	reply, err := h.receive()
	if err != nil {
		return err
	}
	login, ok := reply.(*packet.TanLoginResponse)
	if !ok {
		return fmt.Errorf("房间节点返回了非登录响应")
	}
	if login.ErrorCode != 0 {
		return fmt.Errorf("房间节点拒绝登录，错误码 %d", login.ErrorCode)
	}
	if err := h.encoder.EnableEncryption(info.EncryptKey, info.DecryptKey); err != nil {
		return err
	}
	return h.decoder.EnableEncryption(info.EncryptKey, info.DecryptKey)
}

func (h *Host) createRoom(profile config.Profile, info auth.RoomInfo) error {
	privacy := uint8(0)
	if profile.FriendsOnly {
		privacy = 1
	}
	if err := h.send(&packet.TanCreateRoomRequest{
		Capacity: uint8(profile.MaxPlayers), Privacy: privacy, Slogan: profile.RoomName, Password: profile.RoomPassword,
		Tips: encoding.RoomTips{
			LevelID: levelID, ConstantTestString: "test",
			ProtocolID: info.ProtocolID, GameVersion: profile.RoomGameVersion,
		},
		MinLevel: profile.MinLevel,
		PvP:      profile.AllowPvP, PlayerAuth: 1, EnableWebRTC: true, OwnerPing: 3, PerfLv: 1,
	}); err != nil {
		return err
	}
	reply, err := h.receive()
	if err != nil {
		return err
	}
	created, ok := reply.(*packet.TanCreateRoomResponse)
	if !ok {
		return fmt.Errorf("房间节点返回了非开房响应")
	}
	if created.ErrorCode != 0 {
		return fmt.Errorf("创建房间失败，错误码 %d（7 表示名称需要 VIP）", created.ErrorCode)
	}
	if h.ID = created.RoomID; h.ID == 0 {
		return fmt.Errorf("房间节点返回了空房间号")
	}
	if len(profile.RoomTags) > 0 {
		if err := h.send(&packet.TanSetTagListRequest{TagList: profile.RoomTags}); err != nil {
			return fmt.Errorf("设置房间标签失败: %w", err)
		}
		if h.trace != nil {
			h.trace(fmt.Sprintf("已请求设置房间标签：%v", profile.RoomTags))
		}
	}
	return nil
}

func (h *Host) openSignaling(ctx context.Context, info auth.RoomInfo) error {
	// The signaling package owns its keepalive (ping every 50s, TURN refresh
	// every 30min); the room survives credential refreshes.
	conn, err := (signaling.Dialer{
		NetherNetID:  h.networkID,
		RefreshTime:  30 * time.Minute,
		DebugSignals: true,
		Logf: func(line string) {
			if h.trace != nil {
				h.trace("信令：" + line)
			}
		},
	}).DialContext(ctx, info.SignalingAddress, info.UserID, info.SignalingSeed, info.SignalingTicket)
	if err != nil {
		return fmt.Errorf("WebRTC 信令连接失败（连接超时、节点拒绝或信令格式不兼容）")
	}
	h.signaling = conn
	return nil
}

func (h *Host) startListener(ctx context.Context, trace func(string)) error {
	settings := webrtc.SettingEngine{LoggerFactory: pionLogger{trace: trace}}
	listener, err := (nethernet.ListenConfig{
		API: webrtc.NewAPI(webrtc.WithSettingEngine(settings)),
		Log: slog.New(webrtcLogger{trace: trace}),
		ConnContext: func(parent context.Context, _ *nethernet.Conn) context.Context {
			child, cancel := context.WithTimeout(parent, 30*time.Second)
			context.AfterFunc(child, cancel)
			return child
		},
	}).Listen(signalingTrace{Signaling: h.signaling, trace: trace})
	if err != nil {
		return err
	}
	h.listener = listener
	return nil
}
