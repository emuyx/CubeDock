package room

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/encoding"
)

// TestRoomTipsEncoding guards the wire shape against the captured official
// client payload. The LevelID itself is arbitrary; the field order and sizes
// are not.
func TestRoomTipsEncoding(t *testing.T) {
	captured, err := hex.DecodeString("000c336b4342557573796c6c6b3d0000047465737400002a000a312e32312e3132302e30")
	if err != nil {
		t.Fatal(err)
	}
	tips := encoding.RoomTips{
		LevelID: "3kCBUusyllk=", ConstantTestString: "test",
		ProtocolID: 42, GameVersion: "1.21.120.0",
	}
	buffer := bytes.NewBuffer(nil)
	encoding.NewWriter(buffer, 0).RoomTips(&tips)
	if !bytes.Equal(buffer.Bytes()[2:], captured) {
		t.Fatalf("room tips differ from captured client: %x", buffer.Bytes()[2:])
	}
	var roundTrip encoding.RoomTips
	encoding.NewReader(bytes.NewBuffer(buffer.Bytes())).RoomTips(&roundTrip)
	if roundTrip != tips {
		t.Fatalf("reader lost fields: %#v", roundTrip)
	}
}
