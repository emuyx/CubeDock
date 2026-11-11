package netease

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// The G79 HTTP envelope is AES-CBC with one of 16 rotating keys. The table is
// protocol material, not expressive implementation; each entry is XORed with
// 0x7C before use.
var envelopeKeyTable = [16]string{
	"1C8D9CAD811F2F1E3F7B3B5D208DBE83",
	"96271EAE017F444B342EB8C53AE106BA",
	"6B5FF1292C60237E776923CF4C49ED9A",
	"F020969C883F9C7ADC2C5A130FD1A9CC",
	"607EB25E875E8C9B5C1C5D08648F2D8F",
	"E67D0B4397CCB39C97A78F03315FBE03",
	"3F8F5F7CC35950BF5C9E2B56B20A1F1B",
	"7B8D1D7D674D7D8D917F7D0F2A4D9B48",
	"78289B0C06234B2A2A7D9D7C3C1C1CD3",
	"180928C6AF6C703FBD1A1C8C7EB06C8F",
	"9D2B5D4D0C843E445C4E2A3E19CCCC3F",
	"ED33B9EF6DCC3D2D45428AF516F80A4A",
	"7B6CBC5C215E3E4B7E2F5F5A206823DD",
	"7932137D6A2E1B7E204D6D891ED55D95",
	"0E516BF59BEE9EB6712F5E5D63AC89D2",
	"ED8BBB2D808A0D8F353F3F7B0E486BE5",
}

const envelopeTailAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func envelopeKey(index byte) ([]byte, error) {
	if int(index) >= len(envelopeKeyTable) {
		return nil, fmt.Errorf("信封密钥索引无效")
	}
	key, err := hex.DecodeString(envelopeKeyTable[index])
	if err != nil {
		return nil, fmt.Errorf("信封密钥格式无效")
	}
	for position := range key {
		key[position] ^= 0x7C
	}
	return key, nil
}

// encryptEnvelope builds the hex body sent to PE authentication. random may be
// crypto/rand.Reader; tests substitute a deterministic reader.
func encryptEnvelope(plaintext []byte, random io.Reader) (string, error) {
	if random == nil {
		random = rand.Reader
	}
	tailSource := make([]byte, 16)
	if _, err := io.ReadFull(random, tailSource); err != nil {
		return "", fmt.Errorf("生成信封尾缀失败: %w", err)
	}
	tail := make([]byte, 16)
	for index, value := range tailSource {
		tail[index] = envelopeTailAlphabet[int(value)%len(envelopeTailAlphabet)]
	}
	body := append(append([]byte(nil), plaintext...), tail...)
	if remainder := len(body) % aes.BlockSize; remainder != 0 {
		body = append(body, make([]byte, aes.BlockSize-remainder)...)
	}
	var selector [17]byte
	if _, err := io.ReadFull(random, selector[:]); err != nil {
		return "", fmt.Errorf("生成信封随机参数失败: %w", err)
	}
	index := selector[0] % 16
	iv := selector[1:17]
	key, err := envelopeKey(index)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	encrypted := make([]byte, len(body))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, body)
	result := append(append([]byte(nil), iv...), encrypted...)
	result = append(result, index<<4|0x4)
	return hex.EncodeToString(result), nil
}

func decryptEnvelope(hexText string) ([]byte, error) {
	raw, err := hex.DecodeString(hexText)
	if err != nil {
		return nil, fmt.Errorf("信封不是有效的十六进制")
	}
	if len(raw) < 17+16 {
		return nil, fmt.Errorf("信封长度无效")
	}
	info := raw[len(raw)-1]
	if info&0x0F != 0x4 {
		return nil, fmt.Errorf("信封版本不受支持")
	}
	index := info >> 4
	key, err := envelopeKey(index)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ciphertext := raw[16 : len(raw)-1]
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("信封密文长度无效")
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, raw[:16]).CryptBlocks(plain, ciphertext)
	last := len(plain) - 1
	for last >= 0 && plain[last] == 0 {
		last--
	}
	if last < 16 {
		return nil, fmt.Errorf("信封内容无效")
	}
	return plain[:last-16+1], nil
}
