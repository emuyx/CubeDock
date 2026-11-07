package bridge

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"strings"
)

// ParseIdentity verifies a Bedrock login certificate chain and returns the
// identity claims plus the signed skin token. CryptoRelay uses it to preserve
// the client's NetEase identity while re-signing the login.
func ParseIdentity(request []byte) (map[string]any, map[string]json.RawMessage, error) {
	readString := func(p []byte) (string, []byte, error) {
		if len(p) < 4 {
			return "", nil, io.ErrUnexpectedEOF
		}
		size := uint64(binary.LittleEndian.Uint32(p))
		if size > uint64(len(p)-4) {
			return "", nil, io.ErrUnexpectedEOF
		}
		return string(p[4 : 4+int(size)]), p[4+int(size):], nil
	}
	identityText, rest, err := readString(request)
	if err != nil {
		return nil, nil, err
	}
	skinToken, _, err := readString(rest)
	if err != nil {
		return nil, nil, err
	}
	var envelope struct {
		Certificate string
		Chain       []string `json:"chain"`
	}
	if json.Unmarshal([]byte(identityText), &envelope) != nil {
		return nil, nil, errors.New("身份封装不是 JSON")
	}
	if envelope.Certificate != "" {
		var cert struct {
			Chain []string `json:"chain"`
		}
		if json.Unmarshal([]byte(envelope.Certificate), &cert) != nil {
			return nil, nil, errors.New("Certificate 格式无效")
		}
		envelope.Chain = cert.Chain
	}
	if len(envelope.Chain) == 0 {
		return nil, nil, errors.New("缺少玩家证书链")
	}
	var last map[string]json.RawMessage
	var key *ecdsa.PublicKey
	for i, token := range envelope.Chain {
		header, claims, err := jwtParts(token)
		if err != nil {
			return nil, nil, err
		}
		if i == 0 {
			var pub string
			json.Unmarshal(header["x5u"], &pub)
			key, err = parseGamePublicKey(pub)
			if err != nil {
				return nil, nil, err
			}
		}
		if err = verifyGameJWT(token, key); err != nil {
			return nil, nil, errors.New("玩家证书链签名无效")
		}
		var next string
		if json.Unmarshal(claims["identityPublicKey"], &next) == nil && next != "" {
			key, err = parseGamePublicKey(next)
			if err != nil {
				return nil, nil, err
			}
		}
		last = claims
	}
	if err = verifyGameJWT(skinToken, key); err != nil {
		return nil, nil, errors.New("玩家皮肤签名无效")
	}
	_, skin, err := jwtParts(skinToken)
	if err != nil {
		return nil, nil, err
	}
	var identity map[string]any
	identityReader := json.NewDecoder(bytes.NewReader(last["extraData"]))
	identityReader.UseNumber()
	if identityReader.Decode(&identity) != nil || identity["identity"] == nil || identity["displayName"] == nil {
		return nil, nil, errors.New("玩家证书缺少名称或 UUID")
	}
	return identity, skin, nil
}

func jwtParts(token string) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, errors.New("JWT 格式无效")
	}
	var objects [2]map[string]json.RawMessage
	for i := 0; i < 2; i++ {
		raw, err := base64.RawURLEncoding.DecodeString(parts[i])
		if err != nil || json.Unmarshal(raw, &objects[i]) != nil {
			return nil, nil, errors.New("JWT 内容格式无效")
		}
	}
	return objects[0], objects[1], nil
}

func parseGamePublicKey(encoded string) (*ecdsa.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("公钥编码无效")
	}
	parsed, err := x509.ParsePKIXPublicKey(raw)
	if err != nil {
		return nil, errors.New("公钥格式无效")
	}
	key, ok := parsed.(*ecdsa.PublicKey)
	if !ok || key.Curve != elliptic.P384() {
		return nil, errors.New("需要 P384 公钥")
	}
	return key, nil
}

func verifyGameJWT(token string, key *ecdsa.PublicKey) error {
	header, _, err := jwtParts(token)
	if err != nil {
		return err
	}
	var alg string
	json.Unmarshal(header["alg"], &alg)
	if alg != "ES384" {
		return errors.New("签名算法不是 ES384")
	}
	parts := strings.Split(token, ".")
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 96 {
		return errors.New("签名长度无效")
	}
	hash := sha512.Sum384([]byte(parts[0] + "." + parts[1]))
	if !ecdsa.Verify(key, hash[:], new(big.Int).SetBytes(sig[:48]), new(big.Int).SetBytes(sig[48:])) {
		return errors.New("签名校验失败")
	}
	return nil
}

func SignJWT(key *ecdsa.PrivateKey, claims any) (string, error) {
	rawKey, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	header, _ := json.Marshal(map[string]any{"alg": "ES384", "x5u": base64.StdEncoding.EncodeToString(rawKey)})
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	content := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	hash := sha512.Sum384([]byte(content))
	r, s, err := ecdsa.Sign(rand.Reader, key, hash[:])
	if err != nil {
		return "", err
	}
	signature := append(r.FillBytes(make([]byte, 48)), s.FillBytes(make([]byte, 48))...)
	return content + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

type gameCipher struct {
	stream  cipher.Stream
	key     [32]byte
	counter uint64
}

func newGameCipher(key [32]byte) *gameCipher {
	block, _ := aes.NewCipher(key[:])
	iv := append(append([]byte(nil), key[:12]...), 0, 0, 0, 2)
	return &gameCipher{stream: cipher.NewCTR(block, iv), key: key}
}

func (c *gameCipher) checksum(data []byte) []byte {
	h := sha256.New()
	h.Write(binary.LittleEndian.AppendUint64(nil, c.counter))
	h.Write(data)
	h.Write(c.key[:])
	c.counter++
	return h.Sum(nil)[:8]
}

func (c *gameCipher) encrypt(data []byte) []byte {
	out := append([]byte(nil), data...)
	out = append(out, c.checksum(data)...)
	c.stream.XORKeyStream(out, out)
	return out
}

func (c *gameCipher) decrypt(data []byte) ([]byte, error) {
	if len(data) < 8 {
		return nil, errors.New("加密批次太短")
	}
	out := append([]byte(nil), data...)
	c.stream.XORKeyStream(out, out)
	plain := out[:len(out)-8]
	if !bytes.Equal(c.checksum(plain), out[len(out)-8:]) {
		return nil, errors.New("本地服务器游戏包校验失败")
	}
	return plain, nil
}
