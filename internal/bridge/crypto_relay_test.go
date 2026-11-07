package bridge

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/emuyx/cubedock/internal/gamebatch"
	"github.com/emuyx/cubedock/internal/relay/relaytest"
)

func testLogin(t *testing.T, identity map[string]any) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	publicRaw, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	publicText := base64.StdEncoding.EncodeToString(publicRaw)
	certificate, err := SignJWT(key, map[string]any{"extraData": identity, "identityPublicKey": publicText})
	if err != nil {
		t.Fatal(err)
	}
	chain, _ := json.Marshal(map[string]any{"chain": []string{certificate}})
	envelope, _ := json.Marshal(map[string]any{"Certificate": string(chain), "AuthenticationType": 2, "Token": ""})
	skin, err := SignJWT(key, map[string]any{"GameVersion": "1.21.124", "SkinId": "relay-test"})
	if err != nil {
		t.Fatal(err)
	}
	request := binary.LittleEndian.AppendUint32(nil, uint32(len(envelope)))
	request = append(request, envelope...)
	request = binary.LittleEndian.AppendUint32(request, uint32(len(skin)))
	request = append(request, skin...)
	packet := []byte{1, 0, 0, 3, 0x5c}
	packet = binary.AppendUvarint(packet, uint64(len(request)))
	packet = append(packet, request...)
	return append([]byte{0xfe, 0xff}, gamebatch.Join([][]byte{packet})...)
}

func TestCryptoRelayRepackagesLoginPreservingIdentity(t *testing.T) {
	stream := relaytest.New()
	var traces []string
	relay, err := NewCryptoRelay(stream, func(line string) { traces = append(traces, line) })
	if err != nil {
		t.Fatal(err)
	}
	identity := map[string]any{
		"identity":     uuid.NewString(),
		"displayName":  "EMYX1567",
		"netease_uid":  "1261597877",
		"netease_uuid": uuid.NewString(),
		"XUID":         "",
	}
	login := testLogin(t, identity)
	if _, err := relay.Write([]byte{0xfe, 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := relay.Write(login); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Next(time.Second); err != nil {
		t.Fatalf("first relay write: %v", err)
	}
	wire, err := stream.Next(time.Second)
	if err != nil {
		t.Fatalf("second relay write: %v", err)
	}

	plain, err := gamebatch.Decode(wire[1:])
	if err != nil {
		t.Fatal(err)
	}
	packets, err := gamebatch.Split(plain)
	if err != nil || len(packets) != 1 {
		t.Fatalf("login batch = %d packets, %v", len(packets), err)
	}
	packet := packets[0]
	_, n := binary.Uvarint(packet)
	size, m := binary.Uvarint(packet[n+4:])
	if m <= 0 {
		t.Fatal("login length missing")
	}
	request := packet[n+4+m : n+4+m+int(size)]
	got, _, err := ParseIdentity(request)
	if err != nil {
		t.Fatalf("rewritten login does not verify: %v", err)
	}
	if got["displayName"] != "EMYX1567" || got["netease_uid"] != "1261597877" ||
		got["netease_uuid"] != identity["netease_uuid"] || got["XUID"] != "" {
		t.Fatalf("identity fields changed: %v", got)
	}

	envelopeSize := binary.LittleEndian.Uint32(request)
	identityText := string(request[4 : 4+envelopeSize])
	var envelope struct{ Certificate string }
	if err := json.Unmarshal([]byte(identityText), &envelope); err != nil {
		t.Fatal(err)
	}
	var chain struct {
		Chain []string `json:"chain"`
	}
	if err := json.Unmarshal([]byte(envelope.Certificate), &chain); err != nil || len(chain.Chain) != 1 {
		t.Fatalf("relay chain = %d elements, %v", len(chain.Chain), err)
	}
	if len(traces) == 0 {
		t.Fatal("relay did not trace the login rewrite")
	}
}
