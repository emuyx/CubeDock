package bridge

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/emuyx/cubedock/internal/gamebatch"
	"github.com/emuyx/cubedock/internal/relay"
)

// CryptoRelay forwards a NetEase local-room login to an upstream that performs
// the standard Bedrock encryption handshake. A local-room client never answers
// ServerToClientHandshake, so the relay re-signs the login with its own key
// while preserving the original NetEase identity fields, completes the
// handshake as the client, and encrypts/decrypts game batches. Packet payloads
// are otherwise forwarded unchanged.
type CryptoRelay struct {
	relay.Stream
	key     *ecdsa.PrivateKey
	send    *gameCipher
	recv    *gameCipher
	trace   func(string)
	writeMu sync.Mutex
	reads   int
	writes  int
	closed  atomic.Bool

	playerMu     sync.Mutex
	player       PlayerInfo
	joined       bool
	statusSeen   bool
	inspectReads int
	onLogin      func(PlayerInfo)
	onPlayStatus func(PlayerInfo, int32)
}

func NewCryptoRelay(server relay.Stream, trace func(string)) (*CryptoRelay, error) {
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &CryptoRelay{Stream: server, key: key, trace: trace}, nil
}

func (r *CryptoRelay) Write(clientBatch []byte) (int, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if r.closed.Load() {
		return 0, io.ErrClosedPipe
	}

	size := len(clientBatch)
	r.writes++
	if r.writes == 2 {
		rewritten, err := r.resignLogin(clientBatch)
		if err != nil {
			return 0, fmt.Errorf("网易上游登录重签失败：%w", err)
		}
		clientBatch = rewritten
		if r.trace != nil {
			r.trace("已重签网易登录链以完成上游加密握手，保留原始网易身份字段")
		}
	}
	if err := r.sendToUpstream(clientBatch); err != nil {
		return 0, err
	}
	return size, nil
}

func (r *CryptoRelay) sendToUpstream(batch []byte) error {
	if r.send != nil {
		if len(batch) == 0 || batch[0] != 0xfe {
			return errors.New("缺少 RakNet 游戏包头")
		}
		batch = append([]byte{0xfe}, r.send.encrypt(batch[1:])...)
	}
	written, err := r.Stream.Write(batch)
	if err != nil {
		return err
	}
	if written != len(batch) {
		return io.ErrShortWrite
	}
	return nil
}

func (r *CryptoRelay) ReadPacket() ([]byte, error) {
	for {
		packet, err := r.Stream.ReadPacket()
		if err != nil {
			return nil, err
		}
		if len(packet) == 0 || packet[0] != 0xfe {
			continue
		}
		r.reads++
		if r.recv != nil {
			plain, err := r.recv.decrypt(packet[1:])
			if err != nil {
				return nil, err
			}
			out := append([]byte{0xfe}, plain...)
			r.inspectServerPacket(out)
			return out, nil
		}
		if r.reads != 2 {
			r.inspectServerPacket(packet)
			return packet, nil
		}
		// The second server batch is ServerToClientHandshake. Terminate it here;
		// the local-room client must never see it.
		out, again, err := r.handleServerHandshake(packet)
		if err != nil {
			return nil, err
		}
		if again {
			continue
		}
		return out, nil
	}
}

func (r *CryptoRelay) handleServerHandshake(packet []byte) ([]byte, bool, error) {
	decoded, err := gamebatch.Decode(packet[1:])
	if err != nil {
		return nil, false, err
	}
	packets, err := gamebatch.Split(decoded)
	if err != nil {
		return nil, false, err
	}

	remaining := make([][]byte, 0, len(packets))
	handled := false
	for _, raw := range packets {
		packetID, offset := binary.Uvarint(raw)
		if offset <= 0 || packetID&0x3ff != 3 {
			remaining = append(remaining, raw)
			continue
		}
		token, err := handshakeToken(raw[offset:])
		if err != nil {
			return nil, false, err
		}
		if err := r.acceptServerHandshake(token); err != nil {
			return nil, false, err
		}
		handled = true
	}
	if !handled {
		r.inspectServerPacket(packet)
		return packet, false, nil
	}
	if len(remaining) == 0 {
		return nil, true, nil
	}
	out := append([]byte{0xfe, 0xff}, gamebatch.Join(remaining)...)
	r.inspectServerPacket(out)
	return out, false, nil
}

func handshakeToken(payload []byte) (string, error) {
	size, offset := binary.Uvarint(payload)
	if offset <= 0 || size > uint64(len(payload)-offset) {
		return "", errors.New("服务器加密握手格式错误")
	}
	return string(payload[offset : offset+int(size)]), nil
}

func (r *CryptoRelay) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}
	return r.Stream.Close()
}
