package interoplogin

import (
	"crypto/aes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestExtractVerifyURL(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"plain", "请访问 https://example.com/verify?token=abc 完成验证", "https://example.com/verify?token=abc"},
		{"quoted", `前往 "https://example.com/v?x=1" 验证`, "https://example.com/v?x=1"},
		{"trailing punctuation", "见(https://example.com/v). 然后重试", "https://example.com/v"},
		{"chinese punctuation", "https://example.com/v，请完成验证", "https://example.com/v"},
		{"no link", "网易要求安全验证", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractVerifyURL(tc.text); got != tc.want {
				t.Fatalf("extractVerifyURL(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestVerifyRequiredErrorIncludesURL(t *testing.T) {
	err := &VerifyRequired{Message: "需要验证", URL: "https://example.com/v"}
	got := err.Error()
	if !strings.Contains(got, "需要验证") || !strings.Contains(got, "https://example.com/v") {
		t.Fatalf("Error() = %q", got)
	}
}

func TestSystemVersionOverride(t *testing.T) {
	t.Setenv("CUBEDOCK_INTEROP_SYSTEM_VERSION", "10.0.99999")
	if got := systemVersion(); got != "10.0.99999" {
		t.Fatalf("systemVersion() = %q, want env override", got)
	}
}

func TestEncryptParamsRoundTrip(t *testing.T) {
	const keyHex = "000102030405060708090a0b0c0d0e0f"
	const uniqueID = "unique-device-id"
	const username = "user@example.com"
	passwordMD5 := fmt.Sprintf("%x", md5.Sum([]byte("secret-password")))

	encoded, err := encryptParams(keyHex, uniqueID, username, passwordMD5)
	if err != nil {
		t.Fatalf("encryptParams: %v", err)
	}
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}
	if len(raw) == 0 || len(raw)%aes.BlockSize != 0 {
		t.Fatalf("ciphertext length %d is not a multiple of the AES block size", len(raw))
	}

	key, _ := hex.DecodeString(keyHex)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}
	plain := make([]byte, len(raw))
	for start := 0; start < len(raw); start += aes.BlockSize {
		block.Decrypt(plain[start:start+aes.BlockSize], raw[start:start+aes.BlockSize])
	}
	pad := int(plain[len(plain)-1])
	if pad == 0 || pad > aes.BlockSize || pad > len(plain) {
		t.Fatalf("invalid PKCS#7 padding %d", pad)
	}
	plain = plain[:len(plain)-pad]

	var payload map[string]string
	if err := json.Unmarshal(plain, &payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload["password"] != passwordMD5 {
		t.Fatalf("password field = %q, want MD5 %q", payload["password"], passwordMD5)
	}
	if payload["unique_id"] != uniqueID || payload["username"] != username {
		t.Fatalf("payload identity fields = %#v", payload)
	}
	if strings.Contains(string(plain), "secret-password") {
		t.Fatal("plaintext password leaked into encrypted payload")
	}
}

func TestEncryptParamsRejectsBadKey(t *testing.T) {
	if _, err := encryptParams("not-hex", "id", "user", "md5"); err == nil {
		t.Fatal("expected an error for a non-hex key")
	}
	if _, err := encryptParams("0011223344556677", "id", "user", "md5"); err == nil {
		t.Fatal("expected an error for a short key")
	}
}
