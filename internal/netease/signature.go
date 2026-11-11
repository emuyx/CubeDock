package netease

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"math/bits"
	"strings"
)

const dynamicTokenSalt = "0eGsBkhl"

// DynamicToken signs one HTTP body with the account token. core selects the
// bit-reversed variant used by some gateway endpoints.
func DynamicToken(userToken, path string, body []byte, core bool) string {
	key := md5.Sum([]byte(userToken))
	sum := md5.Sum([]byte(hex.EncodeToString(key[:]) + string(body) + dynamicTokenSalt + strings.TrimSuffix(path, "?")))
	text := []byte(hex.EncodeToString(sum[:]))
	encoded := make([]byte, len(text))
	for index, value := range text {
		rotated := value<<6 | text[(index+1)%len(text)]>>2
		if core {
			rotated = bits.Reverse8(rotated)
		}
		encoded[index] = value ^ rotated
	}
	token := base64.StdEncoding.EncodeToString(encoded)[:16]
	return strings.NewReplacer("+", "m", "/", "o").Replace(token) + "1"
}
