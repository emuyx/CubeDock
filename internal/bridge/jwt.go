package bridge

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
)

// jwtParts decodes the header and payload maps of a three-segment JWT.
func jwtParts(token string) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	segments := strings.Split(token, ".")
	if len(segments) != 3 {
		return nil, nil, errors.New("JWT 格式无效")
	}
	header, err := decodeJWTSegment(segments[0])
	if err != nil {
		return nil, nil, err
	}
	claims, err := decodeJWTSegment(segments[1])
	if err != nil {
		return nil, nil, err
	}
	return header, claims, nil
}

func decodeJWTSegment(segment string) (map[string]json.RawMessage, error) {
	raw, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return nil, errors.New("JWT 内容格式无效")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return nil, errors.New("JWT 内容格式无效")
	}
	return object, nil
}

func parseGamePublicKey(encoded string) (*ecdsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("公钥编码无效")
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
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
	segments := strings.Split(token, ".")
	if len(segments) != 3 {
		return errors.New("JWT 格式无效")
	}
	header, err := decodeJWTSegment(segments[0])
	if err != nil {
		return err
	}
	var algorithm string
	_ = json.Unmarshal(header["alg"], &algorithm)
	if algorithm != "ES384" {
		return errors.New("签名算法不是 ES384")
	}
	signature, err := base64.RawURLEncoding.DecodeString(segments[2])
	if err != nil || len(signature) != 96 {
		return errors.New("签名长度无效")
	}
	digest := sha512.Sum384([]byte(segments[0] + "." + segments[1]))
	left := new(big.Int).SetBytes(signature[:48])
	right := new(big.Int).SetBytes(signature[48:])
	if !ecdsa.Verify(key, digest[:], left, right) {
		return errors.New("签名校验失败")
	}
	return nil
}

// SignJWT creates an ES384 JWT with the relay public key embedded in x5u.
func SignJWT(key *ecdsa.PrivateKey, claims any) (string, error) {
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", err
	}
	header, err := json.Marshal(map[string]any{
		"alg": "ES384",
		"x5u": base64.StdEncoding.EncodeToString(publicDER),
	})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	content := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha512.Sum384([]byte(content))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", err
	}
	signature := append(r.FillBytes(make([]byte, 48)), s.FillBytes(make([]byte, 48))...)
	return content + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}
