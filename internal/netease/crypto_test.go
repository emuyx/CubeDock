package netease

import (
	"bytes"
	"crypto/aes"
	"testing"
)

func decryptECB(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	for offset := 0; offset < len(data); offset += aes.BlockSize {
		block.Decrypt(out[offset:offset+aes.BlockSize], data[offset:offset+aes.BlockSize])
	}
	padding := int(out[len(out)-1])
	return out[:len(out)-padding], nil
}

func TestAESECBPadsAndReverses(t *testing.T) {
	key := md5Token("token")
	plain := []byte("hello")
	sealed, err := aesECBEncrypt(plain, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(sealed)%aes.BlockSize != 0 {
		t.Fatalf("sealed length = %d", len(sealed))
	}
	opened, err := decryptECB(sealed, key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(opened, plain) {
		t.Fatalf("round trip = %q", opened)
	}
	if _, err := aesECBEncrypt(plain, []byte("short")); err == nil {
		t.Fatal("short AES key accepted")
	}
}

func TestDeriveRoomAuth(t *testing.T) {
	random := bytes.NewReader(bytes.Repeat([]byte{1}, 33))
	server := TransferServer{
		IP: "127.0.0.1", SignalWebPort: 18080, Ports: []int{19132, 19133},
	}
	auth, err := DeriveRoomAuth(7, "Bot", "0123456789abcdef", server, random)
	if err != nil {
		t.Fatal(err)
	}
	if auth.RaknetAddress != "127.0.0.1:19133" || auth.SignalingAddress != "127.0.0.1:18080" {
		t.Fatalf("addresses = %s / %s", auth.RaknetAddress, auth.SignalingAddress)
	}
	if len(auth.EncryptKey) != 32 || len(auth.DecryptKey) != 32 ||
		len(auth.RaknetRand) != 16 || len(auth.SignalingSeed) != 16 {
		t.Fatalf("material sizes = %+v", auth)
	}
	if !bytes.Equal(auth.EncryptKey[:16], auth.DecryptKey[16:]) {
		t.Fatal("token half of the key pair does not match")
	}
	if err := ValidateRoomAuth(auth); err != nil {
		t.Fatal(err)
	}
}

func TestSelectTransferServerFiltersUnusableEntries(t *testing.T) {
	servers := []TransferServer{
		{IP: "", SignalWebPort: 1, Ports: []int{1}},
		{IP: "1.2.3.4", SignalWebPort: 0, Ports: []int{1}},
		{IP: "1.2.3.4", SignalWebPort: 1, Ports: nil},
		{IP: "5.6.7.8", SignalWebPort: 18080, Ports: []int{19132}},
	}
	selected, err := SelectTransferServer(servers, bytes.NewReader([]byte{0}))
	if err != nil {
		t.Fatal(err)
	}
	if selected.IP != "5.6.7.8" {
		t.Fatalf("selected = %+v", selected)
	}
	if _, err := SelectTransferServer(servers[:3], bytes.NewReader([]byte{0})); err == nil {
		t.Fatal("unusable list accepted")
	}
}
