package relay

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/emuyx/cubedock/internal/relay/relaytest"
)

func TestDescribeBatchReportsLoginShapeWithoutCredentials(t *testing.T) {
	privateJWT := "sensitive.fixture.jwt"
	identity := []byte(`{"chain":["` + privateJWT + `"]}`)
	connection := binary.LittleEndian.AppendUint32(nil, uint32(len(identity)))
	connection = append(connection, identity...)
	connection = binary.LittleEndian.AppendUint32(connection, uint32(len(privateJWT)))
	connection = append(connection, privateJWT...)
	login := []byte{1, 0, 0, 3, 0x5c}
	login = binary.AppendUvarint(login, uint64(len(connection)))
	login = append(login, connection...)
	batch := binary.AppendUvarint(nil, uint64(len(login)))
	batch = append(batch, login...)

	var compressed bytes.Buffer
	compressed.WriteByte(0)
	writer, _ := flate.NewWriter(&compressed, flate.DefaultCompression)
	_, _ = writer.Write(batch)
	_ = writer.Close()

	description := DescribeBatch(compressed.Bytes(), true)
	if !strings.Contains(description, "chain=true，Certificate=false") {
		t.Fatalf("missing login diagnosis: %s", description)
	}
	if strings.Contains(description, privateJWT) {
		t.Fatal("login description leaked the JWT")
	}
}

func TestDescribeBatchReportsPlayStatus(t *testing.T) {
	description := DescribeBatch([]byte{0xff, 5, 2, 0, 0, 0, 1}, true)
	if !strings.Contains(description, "PlayStatus=1") {
		t.Fatal(description)
	}
	for _, malformed := range [][]byte{{}, {0}, {0xff, 255}, {0xff, 100, 1}, {0, 0, 1}} {
		DescribeBatch(malformed, true) // must not panic
	}
}

func TestObserverCountsAndTracesFirstPackets(t *testing.T) {
	stream := relaytest.New()
	var lines []string
	observer := &Observer{Stream: stream, Label: "test", Trace: func(line string) { lines = append(lines, line) }}
	stream.In <- []byte{0xff, 1}
	if _, err := observer.ReadPacket(); err != nil {
		t.Fatal(err)
	}
	if observer.PacketCount() != 1 || len(lines) != 1 {
		t.Fatalf("count=%d lines=%d", observer.PacketCount(), len(lines))
	}
}
