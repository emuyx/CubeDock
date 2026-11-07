package bridge

import (
	"encoding/binary"
	"testing"

	"github.com/google/uuid"

	"github.com/emuyx/cubedock/internal/gamebatch"
	"github.com/emuyx/cubedock/internal/relay/relaytest"
)

func playStatusPacket(status int32) []byte {
	packet := []byte{2}
	packet = binary.BigEndian.AppendUint32(packet, uint32(status))
	return append([]byte{0xfe, 0xff}, gamebatch.Join([][]byte{packet})...)
}

func TestCryptoRelayReportsPlayerLoginAndSuccess(t *testing.T) {
	stream := relaytest.New()
	relay, err := NewCryptoRelay(stream, nil)
	if err != nil {
		t.Fatal(err)
	}

	var loggedIn PlayerInfo
	var statuses []int32
	relay.SetPlayerHooks(
		func(player PlayerInfo) { loggedIn = player },
		func(_ PlayerInfo, status int32) { statuses = append(statuses, status) },
	)

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
	<-stream.Out
	<-stream.Out

	if loggedIn.Name != "EMYX1567" || loggedIn.UID != "1261597877" {
		t.Fatalf("login hook = %+v", loggedIn)
	}
	if got := relay.Player(); got.Name != "EMYX1567" {
		t.Fatalf("player = %+v", got)
	}
	if relay.Joined() {
		t.Fatal("joined before PlayStatus")
	}

	relay.inspectServerPacket(playStatusPacket(0))
	if !relay.Joined() {
		t.Fatal("PlayStatus LoginSuccess was not recorded")
	}
	if len(statuses) != 1 || statuses[0] != 0 {
		t.Fatalf("statuses = %v", statuses)
	}

	relay.inspectServerPacket(playStatusPacket(0))
	if len(statuses) != 1 {
		t.Fatalf("PlayStatus hook fired twice: %v", statuses)
	}
}

func TestCryptoRelayReadPacketDetectsLoginSuccess(t *testing.T) {
	stream := relaytest.New()
	relay, err := NewCryptoRelay(stream, nil)
	if err != nil {
		t.Fatal(err)
	}
	relay.setPlayer(PlayerInfo{Name: "Alice", UID: "1"})
	var status int32 = -1
	relay.SetPlayerHooks(nil, func(_ PlayerInfo, value int32) { status = value })

	stream.In <- playStatusPacket(0)
	if _, err := relay.ReadPacket(); err != nil {
		t.Fatal(err)
	}
	if !relay.Joined() || status != 0 {
		t.Fatalf("joined=%t status=%d", relay.Joined(), status)
	}
}

func TestCryptoRelayReportsLoginFailure(t *testing.T) {
	relay, err := NewCryptoRelay(relaytest.New(), nil)
	if err != nil {
		t.Fatal(err)
	}
	relay.setPlayer(PlayerInfo{Name: "Alice", UID: "1"})

	var got int32 = -1
	relay.SetPlayerHooks(nil, func(_ PlayerInfo, status int32) { got = status })
	relay.inspectServerPacket(playStatusPacket(7))

	if got != 7 {
		t.Fatalf("status = %d", got)
	}
	if relay.Joined() {
		t.Fatal("login failure marked as joined")
	}
	if name := PlayStatusName(got); name != "服务器已满" {
		t.Fatalf("PlayStatusName = %q", name)
	}
}

func TestPlayerInfoSanitizesDisplayName(t *testing.T) {
	player := playerInfoFromIdentity(map[string]any{
		"displayName": "  Alice\x00\nBuilder  ",
		"netease_uid": float64(123),
	})
	if player.Name != "AliceBuilder" {
		t.Fatalf("name = %q", player.Name)
	}
	if player.UID != "123" {
		t.Fatalf("uid = %q", player.UID)
	}
}
