package bridge

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"time"

	"github.com/emuyx/cubedock/internal/gamebatch"
)

// resignLogin replaces the client's single self-signed chain with a chain
// signed by the relay key. The identity map and client data claims are kept
// unchanged, so the upstream sees the same NetEase profile while the relay can
// complete the ECDH handshake the client will not perform.
func (r *CryptoRelay) resignLogin(wire []byte) ([]byte, error) {
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
	for index, raw := range packets {
		packetID, idLength := binary.Uvarint(raw)
		if idLength <= 0 || packetID&0x3ff != 1 {
			continue
		}
		prefix, request, err := splitLoginPacket(raw, idLength)
		if err != nil {
			return nil, err
		}
		identity, skin, err := ParseIdentity(request)
		if err != nil {
			return nil, err
		}
		translated, err := r.buildResignedEnvelope(identity, skin)
		if err != nil {
			return nil, err
		}
		packets[index] = append(append([]byte(nil), prefix...), appendUvarint(nil, translated)...)
		loginPlayer = playerInfoFromIdentity(identity)
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

func splitLoginPacket(raw []byte, idLength int) ([]byte, []byte, error) {
	prefixEnd := idLength + 4
	if len(raw) < prefixEnd {
		return nil, nil, errors.New("登录协议字段不完整")
	}
	length, lengthBytes := binary.Uvarint(raw[prefixEnd:])
	if lengthBytes <= 0 || length > uint64(len(raw)-prefixEnd-lengthBytes) {
		return nil, nil, errors.New("登录请求长度无效")
	}
	requestStart := prefixEnd + lengthBytes
	prefix := raw[:prefixEnd]
	request := raw[requestStart : requestStart+int(length)]
	return prefix, request, nil
}

func appendUvarint(dst, payload []byte) []byte {
	dst = binary.AppendUvarint(dst, uint64(len(payload)))
	return append(dst, payload...)
}

func (r *CryptoRelay) buildResignedEnvelope(identity map[string]any, skin map[string]json.RawMessage) ([]byte, error) {
	publicDER, err := x509.MarshalPKIXPublicKey(&r.key.PublicKey)
	if err != nil {
		return nil, err
	}
	publicText := base64.StdEncoding.EncodeToString(publicDER)
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
	chainJSON, err := json.Marshal(map[string]any{"chain": []string{certificate}})
	if err != nil {
		return nil, err
	}
	envelope, err := json.Marshal(map[string]any{
		"Certificate": string(chainJSON), "AuthenticationType": 2, "Token": "",
	})
	if err != nil {
		return nil, err
	}
	translated := binary.LittleEndian.AppendUint32(nil, uint32(len(envelope)))
	translated = append(translated, envelope...)
	translated = binary.LittleEndian.AppendUint32(translated, uint32(len(skinJWT)))
	return append(translated, skinJWT...), nil
}
