package room

import (
	"bytes"
	"testing"

	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/encoding"
)

// TestRoomTipsRoundTrip covers the field order and string encoding used when a
// room is advertised to the transfer node.
func TestRoomTipsRoundTrip(t *testing.T) {
	tips := encoding.RoomTips{
		LevelID:            "CubeDock",
		GameType:           0,
		ConstantTestString: "test",
		ProtocolID:         43,
		GameVersion:        "1.21.130.0",
	}

	var wire bytes.Buffer
	encoding.NewWriter(&wire, 0).RoomTips(&tips)
	if wire.Len() <= len(tips.LevelID) {
		t.Fatalf("encoded room tips are suspiciously short: %d bytes", wire.Len())
	}

	var decoded encoding.RoomTips
	encoding.NewReader(bytes.NewReader(wire.Bytes())).RoomTips(&decoded)
	if decoded != tips {
		t.Fatalf("round trip lost fields: %#v", decoded)
	}
}
