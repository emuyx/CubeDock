package auth

import (
	"encoding/base64"
	"testing"
)

func TestUnwrapToken(t *testing.T) {
	raw := `{"sauth_json":"{}"}`
	if got, err := unwrapToken(raw); err != nil || got != raw {
		t.Fatalf("raw = %q, %v", got, err)
	}
	encoded := "b64:" + base64.StdEncoding.EncodeToString([]byte(raw))
	if got, err := unwrapToken(encoded); err != nil || got != raw {
		t.Fatalf("encoded = %q, %v", got, err)
	}
	if _, err := unwrapToken("   "); err == nil {
		t.Fatal("empty token accepted")
	}
	if _, err := unwrapToken("b64:%%%"); err == nil {
		t.Fatal("invalid base64 accepted")
	}
}

func validRoomInfo() RoomInfo {
	return RoomInfo{
		RaknetRand:      make([]byte, 16),
		RaknetAESRand:   make([]byte, 16),
		SignalingSeed:   make([]byte, 16),
		SignalingTicket: make([]byte, 16),
		EncryptKey:      make([]byte, 32),
		DecryptKey:      make([]byte, 32),
	}
}

func TestValidateRoomInfo(t *testing.T) {
	if err := validateRoomInfo(validRoomInfo()); err != nil {
		t.Fatalf("valid room info rejected: %v", err)
	}
	cases := []struct {
		name string
		edit func(*RoomInfo)
	}{
		{"raknet rand", func(i *RoomInfo) { i.RaknetRand = make([]byte, 15) }},
		{"raknet aes rand", func(i *RoomInfo) { i.RaknetAESRand = make([]byte, 15) }},
		{"signaling seed", func(i *RoomInfo) { i.SignalingSeed = nil }},
		{"signaling ticket", func(i *RoomInfo) { i.SignalingTicket = make([]byte, 8) }},
		{"encrypt key", func(i *RoomInfo) { i.EncryptKey = make([]byte, 16) }},
		{"decrypt key", func(i *RoomInfo) { i.DecryptKey = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := validRoomInfo()
			tc.edit(&info)
			if err := validateRoomInfo(info); err == nil {
				t.Fatalf("invalid %s accepted", tc.name)
			}
		})
	}
}
