package bridge

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// acceptServerHandshake completes the upstream ECDH handshake on behalf of the
// local-room client, which never answers ServerToClientHandshake itself.
func (r *CryptoRelay) acceptServerHandshake(token string) error {
	header, claims, err := jwtParts(token)
	if err != nil {
		return err
	}
	var publicText string
	if json.Unmarshal(header["x5u"], &publicText) != nil || publicText == "" {
		return errors.New("服务器未提供握手公钥")
	}
	public, err := parseGamePublicKey(publicText)
	if err != nil {
		return err
	}
	if err := verifyGameJWT(token, public); err != nil {
		return errors.New("服务器握手签名无效")
	}
	var saltText string
	if json.Unmarshal(claims["salt"], &saltText) != nil || saltText == "" {
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
	if _, err := r.Stream.Write(handshake); err != nil {
		return err
	}
	if r.trace != nil {
		r.trace("上游 Bedrock 加密握手已完成（代本地联机客户端）")
	}
	return nil
}
