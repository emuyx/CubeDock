package bridge

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

type gameCipher struct {
	stream   cipher.Stream
	secret   [32]byte
	sequence uint64
}

func newGameCipher(secret [32]byte) *gameCipher {
	block, _ := aes.NewCipher(secret[:])
	iv := make([]byte, aes.BlockSize)
	copy(iv, secret[:12])
	iv[aes.BlockSize-1] = 2
	return &gameCipher{stream: cipher.NewCTR(block, iv), secret: secret}
}

func (c *gameCipher) digest(payload []byte) []byte {
	hash := sha256.New()
	var counter [8]byte
	binary.LittleEndian.PutUint64(counter[:], c.sequence)
	hash.Write(counter[:])
	hash.Write(payload)
	hash.Write(c.secret[:])
	c.sequence++
	sum := hash.Sum(nil)
	return sum[:8]
}

func (c *gameCipher) encrypt(payload []byte) []byte {
	sealed := append(append([]byte(nil), payload...), c.digest(payload)...)
	c.stream.XORKeyStream(sealed, sealed)
	return sealed
}

func (c *gameCipher) decrypt(sealed []byte) ([]byte, error) {
	if len(sealed) < 8 {
		return nil, errors.New("加密批次太短")
	}
	plainAll := append([]byte(nil), sealed...)
	c.stream.XORKeyStream(plainAll, plainAll)
	payload := plainAll[:len(plainAll)-8]
	if !bytes.Equal(c.digest(payload), plainAll[len(plainAll)-8:]) {
		return nil, errors.New("本地服务器游戏包校验失败")
	}
	return payload, nil
}
