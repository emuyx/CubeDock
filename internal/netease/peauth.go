package netease

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
)

// Interop client baseline. These are protocol facts already verified against
// the official 3.10 client; do not change them without new capture evidence.
const (
	interopEngineVersion = "3.10.5.301486"
	interopPatchVersion  = "3.10.5.301486"
	interopLibraryHash   = "43a8baa6ec5e3df458e707ebfcec65e5"
	interopPatchHash     = "330f7fa238110bf5ed88b5369e7f4504"
	interopSignHash      = "2b3e7ca013bb30a74d822579860c042b"
	interopSignSP        = 2
	interopSignTR        = 8
)

var peAuthConstant = [80]byte{
	0x62, 0x25, 0x1e, 0xf6, 0x40, 0xb3, 0x40, 0xc0, 0x51, 0x5a, 0x5e, 0x26, 0xaa, 0xc7, 0xb6, 0xe9,
	0x44, 0xea, 0xbe, 0xa4, 0xa9, 0xcf, 0xde, 0x4b, 0x60, 0x4b, 0xbb, 0xf6, 0x70, 0xbc, 0xbf, 0xbe,
	0xc3, 0x59, 0x5b, 0x65, 0x92, 0xcc, 0x0c, 0x8f, 0x7d, 0xf4, 0xef, 0xff, 0xd1, 0x5d, 0x84, 0x85,
	0xc6, 0x7e, 0x9b, 0x28, 0xfa, 0x27, 0xa1, 0xea, 0x85, 0x30, 0xef, 0xd4, 0x05, 0x1d, 0x88, 0x04,
	0xe6, 0xcd, 0xe1, 0x21, 0xd6, 0x07, 0x37, 0xc3, 0x87, 0x0d, 0xd5, 0xf4, 0xed, 0x14, 0x5a, 0x45,
}

var peAuthShift = [20]byte{
	0x01, 0x06, 0x0a, 0x0d, 0x02, 0x05, 0x09, 0x0e, 0x04, 0x07, 0x0b, 0x03, 0x03, 0x08, 0x0b, 0x05,
	0x01, 0x07, 0x0b, 0x0e,
}

// peAuthSign reproduces the 16-byte signature sent with the interop PE auth
// payload. sp/tr are the two verified signing parameters (2, 8).
func peAuthSign(value string, sp, tr int) (string, error) {
	if sp < 0 || tr <= 0 {
		return "", fmt.Errorf("签名参数无效")
	}
	if 16*sp+16 > len(peAuthConstant) || 4*sp+4 > len(peAuthShift) {
		return "", fmt.Errorf("签名参数超出常量表范围")
	}
	if remainder := len(value) % 4; remainder != 0 {
		value += strings.Repeat("0", 4-remainder)
	}
	raw := []byte(value)
	words := make([]uint32, 0, len(raw)/4)
	for offset := 0; offset < len(raw); offset += 4 {
		words = append(words, uint32(raw[offset])<<24|uint32(raw[offset+1])<<16|uint32(raw[offset+2])<<8|uint32(raw[offset+3]))
	}
	if remainder := len(words) % 64; remainder != 0 {
		for index := 0; index < 64-remainder; index++ {
			words = append(words, 0xabcde987)
		}
	}

	var constants [4]uint32
	for index := 0; index < 4; index++ {
		offset := 16*sp + index*4
		constants[index] = binary.LittleEndian.Uint32(peAuthConstant[offset : offset+4])
	}
	shifts := peAuthShift[4*sp : 4*sp+4]

	a := uint32(0x67452301)
	b := uint32(0xefcdab89)
	c := uint32(0x98badcfe)
	d := uint32(0x10325476)

	for round := 0; round < tr; round++ {
		// The interop signature consumes every fourth 32-bit word. This is
		// part of the wire behaviour, not an optimisation.
		for index := 0; index < len(words); index += 4 {
			word := words[index]
			a = b + ((a + (b&c | ^b&d) + word + constants[0]) << uint(shifts[0]))
			b = b + ((a + (b&d | ^d&c) + word + constants[1]) << uint(shifts[1]))
			c = b + ((a + (b ^ c ^ d) + word + constants[2]) << uint(shifts[2]))
			d = b + ((a + (c ^ (b | d)) + word + constants[3]) << uint(shifts[3]))
		}
	}

	out := make([]byte, 16)
	binary.LittleEndian.PutUint32(out[0:4], a)
	binary.LittleEndian.PutUint32(out[4:8], b)
	binary.LittleEndian.PutUint32(out[8:12], c)
	binary.LittleEndian.PutUint32(out[12:16], d)
	return base64.StdEncoding.EncodeToString(out), nil
}
