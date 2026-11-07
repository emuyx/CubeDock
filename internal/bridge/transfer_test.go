package bridge

import (
	"encoding/binary"
	"testing"

	"github.com/Happy2018new/nemc-tan-lobby-solver/minecraft/protocol/packet"

	"github.com/emuyx/cubedock/internal/gamebatch"
)

func TestBuildTransferBatch(t *testing.T) {
	batch, err := BuildTransferBatch("example.com", 19132)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch) == 0 || batch[0] != 0xff {
		t.Fatalf("batch marker = %x", batch)
	}
	packets, err := gamebatch.Split(batch[1:])
	if err != nil || len(packets) != 1 {
		t.Fatalf("packets = %d, %v", len(packets), err)
	}
	id, n := binary.Uvarint(packets[0])
	if n <= 0 || id&0x3ff != packet.IDTransfer {
		t.Fatalf("packet id = %d", id)
	}
	payload := packets[0][n:]
	length, m := binary.Uvarint(payload)
	if m <= 0 || length > uint64(len(payload)-m) {
		t.Fatalf("address length = %d", length)
	}
	address := string(payload[m : m+int(length)])
	rest := payload[m+int(length):]
	if address != "example.com" || len(rest) != 3 ||
		binary.LittleEndian.Uint16(rest) != 19132 || rest[2] != 0 {
		t.Fatalf("transfer payload = %q, %x", address, rest)
	}
}

func TestBuildTransferBatchRejectsEmptyValues(t *testing.T) {
	if _, err := BuildTransferBatch("", 19132); err == nil {
		t.Fatal("empty address accepted")
	}
	if _, err := BuildTransferBatch("example.com", 0); err == nil {
		t.Fatal("zero port accepted")
	}
}
