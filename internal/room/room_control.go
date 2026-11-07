package room

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/encoding"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/packet"
)

func (h *Host) send(message packet.Packet) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()

	frame := bytes.NewBuffer(nil)
	header := packet.Header{PacketID: message.ID()}
	if err := header.Write(frame); err != nil {
		return err
	}
	message.Marshal(encoding.NewWriter(frame, 0))
	return h.encoder.Encode(frame.Bytes())
}

func (h *Host) receive() (message packet.Packet, err error) {
	defer func() {
		if recover() != nil {
			message, err = nil, fmt.Errorf("房间协议数据格式错误")
		}
	}()

	raw, err := h.decoder.Decode()
	if err != nil {
		return nil, err
	}
	frame := bytes.NewBuffer(raw)
	header := packet.Header{}
	if err = header.Read(frame); err != nil {
		return nil, err
	}
	message = packet.NewServerPool()[header.PacketID]
	if message == nil {
		if h.trace != nil {
			h.trace(fmt.Sprintf("收到未实现的房间控制包：ID=%d，长度=%d", header.PacketID, len(raw)))
		}
		return nil, nil
	}
	message.Marshal(encoding.NewReader(frame))
	return message, nil
}

func (h *Host) controlLoop() {
	for {
		message, err := h.receive()
		if err != nil {
			h.fail(fmt.Errorf("房间连接断开: %w", err))
			return
		}
		switch event := message.(type) {
		case *packet.TanNewGuestResponse:
			if h.trace != nil {
				h.trace("收到玩家加入请求，发送服务器就绪信息")
			}
			if err = h.notifyReady(); err != nil {
				h.fail(err)
				return
			}
			h.ready.start(event.PlayerID, event.NetherNetID)
		case *packet.TanLeaveRoomResponse:
			h.ready.stopPlayer(event.PlayerID)
		case nil:
		default:
			if h.trace != nil {
				h.trace(fmt.Sprintf("收到房间控制包：%T", event))
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

func (h *Host) notifyReady() error {
	return h.send(&packet.TanNotifyServerReady{
		ServerAddress:         "127.0.0.1|19132",
		NetherNetID:           strconv.FormatUint(h.networkID, 10),
		WebRTCCompressEnabled: true,
	})
}
