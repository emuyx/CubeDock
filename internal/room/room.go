// Package room hosts one NetEase TanLobby room. It speaks the room control
// protocol on a RakNet transfer node and exposes joining players as NetherNet
// connections through a WebRTC signaling session.
package room

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
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

// levelID is the map identifier advertised by the room. Guests only use it to
// pick a loading screen; the Bedrock world comes from the upstream server.
const levelID = "CubeDock"

// Host is one live TanLobby room owned by the bot account.
type Host struct {
	ID        uint32
	control   *raknet.Conn
	encoder   *packet.Encoder
	decoder   *packet.Decoder
	listener  *nethernet.Listener
	signaling *signaling.Conn
	networkID uint64
	trace     func(string)
	ready     *readyQueue
	done      chan struct{}
	closeOnce sync.Once
	writeMu   sync.Mutex
	errMu     sync.Mutex
	err       error
}

// Open authenticates against the transfer node, creates the room and starts
// the signaling session. trace may be nil.
func Open(ctx context.Context, cfg config.Config, info auth.RoomInfo, trace func(string)) (host *Host, err error) {
	var rawID [8]byte
	if _, err = rand.Read(rawID[:]); err != nil {
		return nil, err
	}
	host = &Host{
		networkID: binary.LittleEndian.Uint64(rawID[:]),
		trace:     trace,
		done:      make(chan struct{}),
	}
	if host.networkID == 0 {
		host.networkID = 1
	}
	opened := false
	defer func() {
		if !opened {
			host.Close()
		}
	}()

	dialCtx, stopDial := context.WithTimeout(ctx, 10*time.Second)
	host.control, err = raknet.DialContext(dialCtx, info.RaknetAddress)
	stopDial()
	if err != nil {
		return nil, fmt.Errorf("房间转发节点连接失败: %w", err)
	}
	// Closing the control connection is what cancels registrations, not just
	// the initial UDP handshake.
	stopOnCancel := context.AfterFunc(ctx, func() { host.control.Close() })
	defer stopOnCancel()

	host.encoder = packet.NewEncoder(host.control)
	host.decoder, err = packet.NewDecoder(host.control)
	if err != nil {
		return nil, err
	}
	if err = host.send(&packet.TanLoginRequest{
		PlayerID: info.UserID, Rand: info.RaknetRand, AESRand: info.RaknetAESRand,
		PlayerName: info.PlayerName,
	}); err != nil {
		return nil, err
	}
	reply, err := host.receive()
	if err != nil {
		return nil, err
	}
	login, ok := reply.(*packet.TanLoginResponse)
	if !ok {
		return nil, fmt.Errorf("房间节点返回了非登录响应")
	}
	if login.ErrorCode != 0 {
		return nil, fmt.Errorf("房间节点拒绝登录，错误码 %d", login.ErrorCode)
	}
	if err = host.encoder.EnableEncryption(info.EncryptKey, info.DecryptKey); err != nil {
		return nil, err
	}
	if err = host.decoder.EnableEncryption(info.EncryptKey, info.DecryptKey); err != nil {
		return nil, err
	}

	privacy := uint8(0)
	if cfg.FriendsOnly {
		privacy = 1
	}
	if err = host.send(&packet.TanCreateRoomRequest{
		Capacity: uint8(cfg.MaxPlayers), Privacy: privacy, Slogan: cfg.RoomName, Password: cfg.RoomPassword,
		Tips: encoding.RoomTips{
			LevelID: levelID, ConstantTestString: "test",
			ProtocolID: info.ProtocolID, GameVersion: cfg.RoomGameVersion,
		},
		PvP: cfg.AllowPvP, PlayerAuth: 1, EnableWebRTC: true, OwnerPing: 3, PerfLv: 1,
	}); err != nil {
		return nil, err
	}
	reply, err = host.receive()
	if err != nil {
		return nil, err
	}
	created, ok := reply.(*packet.TanCreateRoomResponse)
	if !ok {
		return nil, fmt.Errorf("房间节点返回了非开房响应")
	}
	if created.ErrorCode != 0 {
		return nil, fmt.Errorf("创建房间失败，错误码 %d（7 表示名称需要 VIP）", created.ErrorCode)
	}
	if host.ID = created.RoomID; host.ID == 0 {
		return nil, fmt.Errorf("房间节点返回了空房间号")
	}

	// The signaling package owns its own keepalive (ping every 50s, TURN
	// refresh every 30min); the room survives credential refreshes.
	host.signaling, err = (signaling.Dialer{
		NetherNetID: host.networkID,
		RefreshTime: 30 * time.Minute,
	}).DialContext(ctx, info.SignalingAddress, info.UserID, info.SignalingSeed, info.SignalingTicket)
	if err != nil {
		return nil, fmt.Errorf("WebRTC 信令连接失败（连接超时、节点拒绝或信令格式不兼容）")
	}
	settings := webrtc.SettingEngine{LoggerFactory: transportLogger{trace: trace}}
	host.listener, err = (nethernet.ListenConfig{
		API: webrtc.NewAPI(webrtc.WithSettingEngine(settings)),
		Log: slog.New(stageLogger{trace: trace}),
		ConnContext: func(parent context.Context, _ *nethernet.Conn) context.Context {
			child, cancel := context.WithTimeout(parent, 30*time.Second)
			context.AfterFunc(child, cancel)
			return child
		},
	}).Listen(signalingObserver{Signaling: host.signaling, trace: trace})
	if err != nil {
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

func (h *Host) send(p packet.Packet) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	buffer := bytes.NewBuffer(nil)
	header := packet.Header{PacketID: p.ID()}
	if err := header.Write(buffer); err != nil {
		return err
	}
	p.Marshal(encoding.NewWriter(buffer, 0))
	return h.encoder.Encode(buffer.Bytes())
}

func (h *Host) receive() (p packet.Packet, err error) {
	defer func() {
		if recover() != nil {
			p, err = nil, fmt.Errorf("房间协议数据格式错误")
		}
	}()
	data, err := h.decoder.Decode()
	if err != nil {
		return nil, err
	}
	buffer := bytes.NewBuffer(data)
	header := packet.Header{}
	if err = header.Read(buffer); err != nil {
		return nil, err
	}
	p = packet.NewServerPool()[header.PacketID]
	if p == nil {
		if h.trace != nil {
			h.trace(fmt.Sprintf("收到未实现的房间控制包：ID=%d，长度=%d", header.PacketID, len(data)))
		}
		return nil, nil
	}
	p.Marshal(encoding.NewReader(buffer))
	return p, nil
}

func (h *Host) controlLoop() {
	for {
		p, err := h.receive()
		if err != nil {
			h.fail(fmt.Errorf("房间连接断开: %w", err))
			return
		}
		switch typed := p.(type) {
		case *packet.TanNewGuestResponse:
			if h.trace != nil {
				h.trace("收到玩家加入请求，发送服务器就绪信息")
			}
			if err = h.notifyReady(); err != nil {
				h.fail(err)
				return
			}
			h.ready.start(typed.PlayerID, typed.NetherNetID)
		case *packet.TanLeaveRoomResponse:
			h.ready.stopPlayer(typed.PlayerID)
		case nil:
		default:
			if h.trace != nil {
				h.trace(fmt.Sprintf("收到房间控制包：%T", typed))
			}
		}
	}
}

func (h *Host) fail(err error) {
	h.errMu.Lock()
	if h.err == nil {
		h.err = err
	}
	h.errMu.Unlock()
	h.Close()
}

// Accept waits for the next guest's NetherNet connection.
func (h *Host) Accept() (net.Conn, error) {
	conn, err := h.listener.Accept()
	if err == nil && conn != nil {
		if addr, ok := conn.RemoteAddr().(*nethernet.Addr); ok {
			h.ready.stopNetwork(strconv.FormatUint(addr.NetworkID, 10))
		}
	}
	return conn, err
}

func (h *Host) notifyReady() error {
	return h.send(&packet.TanNotifyServerReady{
		ServerAddress:         "127.0.0.1|19132",
		NetherNetID:           strconv.FormatUint(h.networkID, 10),
		WebRTCCompressEnabled: true,
	})
}

// Done closes when the control connection ends.
func (h *Host) Done() <-chan struct{} { return h.done }

// Err reports why the room ended, if it failed.
func (h *Host) Err() error {
	h.errMu.Lock()
	defer h.errMu.Unlock()
	return h.err
}

// Close tears down the room. Disconnecting the owner's RakNet session removes
// it from NetEase, so no shutdown packet is invented.
func (h *Host) Close() {
	h.closeOnce.Do(func() {
		close(h.done)
		if h.control != nil {
			h.control.Close()
		}
		if h.listener != nil {
			h.listener.Close()
		} else if h.signaling != nil {
			h.signaling.Close(net.ErrClosed)
		}
	})
}
