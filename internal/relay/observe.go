package relay

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/emuyx/cubedock/internal/redact"
)

// Observer counts packets and describes the first few unencrypted ones. Only
// packet identifiers and lengths are recorded; credential material is never
// emitted.
type Observer struct {
	Stream
	Label   string
	Trace   func(string)
	Packets atomic.Uint64
}

// PacketCount returns the number of packets seen so far. It is safe to call
// while the relay is running.
func (o *Observer) PacketCount() uint64 { return o.Packets.Load() }

func (o *Observer) ReadPacket() ([]byte, error) {
	packet, err := o.Stream.ReadPacket()
	if err != nil {
		return packet, err
	}
	count := o.Packets.Add(1)
	if count > 3 || o.Trace == nil {
		return packet, nil
	}
	game := packet
	if len(game) > 0 && game[0] == 0xfe {
		game = game[1:]
	}
	line := fmt.Sprintf("%s：第 %d 包，长度 %d", o.Label, count, len(packet))
	if len(game) == 7 && game[0] == 6 && game[1] == 0xc1 && game[2] == 1 {
		line += fmt.Sprintf("，RequestNetworkSettings 游戏协议 %d", binary.BigEndian.Uint32(game[3:]))
	} else if len(game) > 0 {
		line += fmt.Sprintf("，批次首字节 0x%02x", game[0])
	}
	line += DescribeBatch(game, count > 1)
	o.Trace(line)
	return packet, nil
}

// DescribeBatch returns a short, credential-free description of a game batch.
// compressed selects whether the body still carries its compression marker.
func DescribeBatch(body []byte, compressed bool) string {
	if len(body) == 0 {
		return ""
	}
	if compressed {
		switch body[0] {
		case 0xff:
			body = body[1:]
		case 0:
			inflater := flate.NewReader(bytes.NewReader(body[1:]))
			defer inflater.Close()
			plain, err := io.ReadAll(io.LimitReader(inflater, 2<<20))
			if err != nil {
				return "，无法解码 Flate 批次"
			}
			body = plain
		default:
			return "，暂不解码此压缩类型"
		}
	}
	description := ""
	for len(body) > 0 {
		size, head := binary.Uvarint(body)
		if head <= 0 || size > uint64(len(body)-head) {
			return description + "，包边界无法解析"
		}
		packet := body[head : head+int(size)]
		body = body[head+int(size):]
		id, n := binary.Uvarint(packet)
		if n <= 0 {
			return description
		}
		payload := packet[n:]
		description += fmt.Sprintf("，PacketID=%d", id&0x3ff)
		switch id & 0x3ff {
		case 143:
			if len(payload) >= 4 {
				description += fmt.Sprintf("(压缩阈值=%d，算法=%d)",
					binary.LittleEndian.Uint16(payload), binary.LittleEndian.Uint16(payload[2:]))
			}
		case 2:
			if len(payload) >= 4 {
				description += fmt.Sprintf("(PlayStatus=%d)", binary.BigEndian.Uint32(payload))
			}
		case 5:
			description += describeDisconnect(payload)
		case 1:
			description += describeLoginFormat(payload)
		}
	}
	return description
}

func describeDisconnect(payload []byte) string {
	reason, n := binary.Varint(payload)
	if n <= 0 || len(payload) <= n {
		return ""
	}
	out := fmt.Sprintf("(断开原因=%d)", reason)
	text := payload[n:]
	if text[0] != 0 {
		return out
	}
	size, head := binary.Uvarint(text[1:])
	if head <= 0 || size > uint64(len(text)-1-head) {
		return out
	}
	return out + "：" + redact.Sanitize(string(text[1+head:1+head+int(size)]), "")
}

func describeLoginFormat(payload []byte) string {
	if len(payload) < 4 {
		return ""
	}
	payload = payload[4:] // protocol version
	size, head := binary.Uvarint(payload)
	if head <= 0 || size > uint64(len(payload)-head) {
		return ""
	}
	request := payload[head : head+int(size)]
	if len(request) < 4 {
		return ""
	}
	length := uint64(binary.LittleEndian.Uint32(request))
	request = request[4:]
	if length > uint64(len(request)) {
		return ""
	}
	var identity map[string]json.RawMessage
	if json.Unmarshal(request[:int(length)], &identity) != nil {
		return ""
	}
	_, legacy := identity["chain"]
	_, modern := identity["Certificate"]
	return fmt.Sprintf("(登录格式：chain=%t，Certificate=%t)", legacy, modern)
}
