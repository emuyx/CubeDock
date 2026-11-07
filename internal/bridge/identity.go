package bridge

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"io"
)

type packetReader struct {
	data []byte
}

func (r *packetReader) nextString() (string, error) {
	if len(r.data) < 4 {
		return "", io.ErrUnexpectedEOF
	}
	size := uint64(r.data[0]) | uint64(r.data[1])<<8 | uint64(r.data[2])<<16 | uint64(r.data[3])<<24
	if size > uint64(len(r.data)-4) {
		return "", io.ErrUnexpectedEOF
	}
	value := string(r.data[4 : 4+size])
	r.data = r.data[4+size:]
	return value, nil
}

// ParseIdentity verifies a Bedrock login certificate chain and returns the
// identity claims plus the signed skin token. CryptoRelay uses it to preserve
// the client's NetEase identity while re-signing the login.
func ParseIdentity(request []byte) (map[string]any, map[string]json.RawMessage, error) {
	reader := &packetReader{data: request}
	identityText, err := reader.nextString()
	if err != nil {
		return nil, nil, err
	}
	skinToken, err := reader.nextString()
	if err != nil {
		return nil, nil, err
	}
	chain, err := certificateChain(identityText)
	if err != nil {
		return nil, nil, err
	}

	var key *ecdsa.PublicKey
	var lastClaims map[string]json.RawMessage
	for index, token := range chain {
		header, claims, err := jwtParts(token)
		if err != nil {
			return nil, nil, err
		}
		if index == 0 {
			key, err = headerPublicKey(header)
			if err != nil {
				return nil, nil, err
			}
		}
		if err := verifyGameJWT(token, key); err != nil {
			return nil, nil, errors.New("玩家证书链签名无效")
		}
		if next := claimString(claims, "identityPublicKey"); next != "" {
			key, err = parseGamePublicKey(next)
			if err != nil {
				return nil, nil, err
			}
		}
		lastClaims = claims
	}

	if err := verifyGameJWT(skinToken, key); err != nil {
		return nil, nil, errors.New("玩家皮肤签名无效")
	}
	_, skin, err := jwtParts(skinToken)
	if err != nil {
		return nil, nil, err
	}
	identity, err := decodeExtraData(lastClaims["extraData"])
	if err != nil || identity["identity"] == nil || identity["displayName"] == nil {
		return nil, nil, errors.New("玩家证书缺少名称或 UUID")
	}
	return identity, skin, nil
}

func certificateChain(text string) ([]string, error) {
	var envelope struct {
		Certificate string
		Chain       []string `json:"chain"`
	}
	if json.Unmarshal([]byte(text), &envelope) != nil {
		return nil, errors.New("身份封装不是 JSON")
	}
	if envelope.Certificate != "" {
		var certificate struct {
			Chain []string `json:"chain"`
		}
		if json.Unmarshal([]byte(envelope.Certificate), &certificate) != nil {
			return nil, errors.New("Certificate 格式无效")
		}
		envelope.Chain = certificate.Chain
	}
	if len(envelope.Chain) == 0 {
		return nil, errors.New("缺少玩家证书链")
	}
	return envelope.Chain, nil
}

func headerPublicKey(header map[string]json.RawMessage) (*ecdsa.PublicKey, error) {
	var encoded string
	if json.Unmarshal(header["x5u"], &encoded) != nil || encoded == "" {
		return nil, errors.New("玩家证书缺少公钥")
	}
	return parseGamePublicKey(encoded)
}

func claimString(claims map[string]json.RawMessage, name string) string {
	var value string
	_ = json.Unmarshal(claims[name], &value)
	return value
}

func decodeExtraData(raw json.RawMessage) (map[string]any, error) {
	var identity map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&identity); err != nil {
		return nil, err
	}
	return identity, nil
}
