package bridge

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

func (r *CryptoRelay) Write(p []byte) (int, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if r.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	original := len(p)
	r.writes++
	if r.writes == 2 {
		var err error
		p, err = r.rewriteLogin(p)
		if err != nil {
			return 0, fmt.Errorf("网易上游登录重签失败：%w", err)
		}
		if r.trace != nil {
			r.trace("已重签网易登录链以完成上游加密握手，保留原始网易身份字段")
		}
	}
	if err := r.writeBackend(p); err != nil {
		return 0, err
	}
	return original, nil
}

func (r *CryptoRelay) writeBackend(p []byte) error {
	if r.send != nil {
		if len(p) == 0 || p[0] != 0xfe {
			return errors.New("缺少 RakNet 游戏包头")
		}
		p = append([]byte{0xfe}, r.send.encrypt(p[1:])...)
	}
	n, err := r.Stream.Write(p)
	if err != nil {
		return err
	}
	if n != len(p) {
		return io.ErrShortWrite
	}
	return nil
}

func (r *CryptoRelay) ReadPacket() ([]byte, error) {
	for {
		p, err := r.Stream.ReadPacket()
		if err != nil {
			return nil, err
		}
		if len(p) == 0 || p[0] != 0xfe {
			continue
		}
		r.reads++
		if r.recv != nil {
			plain, err := r.recv.decrypt(p[1:])
			if err != nil {
				return nil, err
			}
			out := append([]byte{0xfe}, plain...)
			r.inspectServerPacket(out)
			return out, nil
		}
		if r.reads != 2 {
			r.inspectServerPacket(p)
			return p, nil
		}
		// The second server batch is ServerToClientHandshake. Terminate it here;
		// the local-room client must never see it.
		data, err := gamebatch.Decode(p[1:])
		if err != nil {
			return nil, err
		}
		packets, err := gamebatch.Split(data)
		if err != nil {
			return nil, err
		}
		remaining := [][]byte{}
		handled := false
		for _, pk := range packets {
			id, n := binary.Uvarint(pk)
			if n > 0 && id&0x3ff == 3 {
				size, m := binary.Uvarint(pk[n:])
				if m <= 0 || size > uint64(len(pk)-n-m) {
					return nil, errors.New("服务器加密握手格式错误")
				}
				if err := r.enableGameEncryption(string(pk[n+m : n+m+int(size)])); err != nil {
					return nil, err
				}
				handled = true
			} else {
				remaining = append(remaining, pk)
			}
		}
		if !handled {
			r.inspectServerPacket(p)
			return p, nil
		}
		if len(remaining) > 0 {
			out := append([]byte{0xfe, 0xff}, gamebatch.Join(remaining)...)
			r.inspectServerPacket(out)
			return out, nil
		}
	}
}

func (r *CryptoRelay) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}
	return r.Stream.Close()
}

func (r *CryptoRelay) enableGameEncryption(token string) error {
	header, claims, err := jwtParts(token)
	if err != nil {
		return err
	}
	var publicText string
	if json.Unmarshal(header["x5u"], &publicText) != nil {
		return errors.New("服务器未提供握手公钥")
	}
	public, err := parseGamePublicKey(publicText)
	if err != nil {
		return err
	}
	if err = verifyGameJWT(token, public); err != nil {
		return errors.New("服务器握手签名无效")
	}
	var saltText string
	if json.Unmarshal(claims["salt"], &saltText) != nil {
		return errors.New("服务器握手缺少 salt")
	}
	salt, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(saltText, "="))
	if err != nil {
		return errors.New("服务器 salt 格式错误")
	}
	x, _ := public.Curve.ScalarMult(public.X, public.Y, r.key.D.Bytes())
	shared := x.FillBytes(make([]byte, 48))
	secret := sha256.Sum256(append(salt, shared...))
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	r.recv = newGameCipher(secret)
	r.send = newGameCipher(secret)
	handshake := append([]byte{0xfe}, r.send.encrypt([]byte{0xff, 1, 4})...)
	if _, err = r.Stream.Write(handshake); err != nil {
		return err
	}
	if r.trace != nil {
		r.trace("上游 Bedrock 加密握手已完成（代本地联机客户端）")
	}
	return nil
}

// rewriteLogin replaces the client's single self-signed chain with a chain
// signed by the relay key. The identity map and client data claims are kept
// unchanged, so the upstream sees the same NetEase profile while the relay can
// complete the ECDH handshake the client will not perform.
func (r *CryptoRelay) rewriteLogin(wire []byte) ([]byte, error) {
	if len(wire) < 2 || wire[0] != 0xfe {
		return nil, errors.New("登录包缺少 RakNet 包头")
	}
	batch, err := gamebatch.Decode(wire[1:])
	if err != nil {
		return nil, err
	}
	packets, err := gamebatch.Split(batch)
	if err != nil {
		return nil, err
	}
	found := false
	var loginPlayer PlayerInfo
	for index, p := range packets {
		id, n := binary.Uvarint(p)
		if n <= 0 || id&0x3ff != 1 {
			continue
		}
		if len(p) < n+4 {
			return nil, errors.New("登录协议字段不完整")
		}
		prefix := p[:n+4]
		length, m := binary.Uvarint(p[n+4:])
		if m <= 0 || length > uint64(len(p)-n-4-m) {
			return nil, errors.New("登录请求长度无效")
		}
		request := p[n+4+m : n+4+m+int(length)]
		identity, skin, err := ParseIdentity(request)
		if err != nil {
			return nil, err
		}
		loginPlayer = playerInfoFromIdentity(identity)
		publicRaw, _ := x509.MarshalPKIXPublicKey(&r.key.PublicKey)
		publicText := base64.StdEncoding.EncodeToString(publicRaw)
		now := time.Now().Unix()
		certificate, err := SignJWT(r.key, map[string]any{
			"iat": now, "exp": now + 3600, "nbf": now - 60,
			"extraData": identity, "identityPublicKey": publicText,
		})
		if err != nil {
			return nil, err
		}
		skinJWT, err := SignJWT(r.key, skin)
		if err != nil {
			return nil, err
		}
		certJSON, _ := json.Marshal(map[string]any{"chain": []string{certificate}})
		envelope, _ := json.Marshal(map[string]any{"Certificate": string(certJSON), "AuthenticationType": 2, "Token": ""})
		translated := binary.LittleEndian.AppendUint32(nil, uint32(len(envelope)))
		translated = append(translated, envelope...)
		translated = binary.LittleEndian.AppendUint32(translated, uint32(len(skinJWT)))
		translated = append(translated, skinJWT...)
		pk := append([]byte(nil), prefix...)
		pk = binary.AppendUvarint(pk, uint64(len(translated)))
		pk = append(pk, translated...)
		packets[index] = pk
		found = true
	}
	if !found {
		return nil, errors.New("第二个客户端批次没有 Login")
	}
	if loginPlayer.Name != "" || loginPlayer.UID != "" {
		r.setPlayer(loginPlayer)
	}
	return append([]byte{0xfe, 0xff}, gamebatch.Join(packets)...), nil
}
