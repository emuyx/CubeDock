package relay

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/emuyx/cubedock/internal/relay/relaytest"
)

func buildLoginWire(t *testing.T, certificate string) []byte {
	t.Helper()
	identity := []byte(`{"chain":["` + certificate + `"]}`)
	var connection bytes.Buffer
	_ = binary.Write(&connection, binary.LittleEndian, uint32(len(identity)))
	connection.Write(identity)
	_ = binary.Write(&connection, binary.LittleEndian, uint32(len(certificate)))
	connection.WriteString(certificate)

	login := []byte{1, 0, 0, 3, 0x5c}
	login = binary.AppendUvarint(login, uint64(connection.Len()))
	login = append(login, connection.Bytes()...)
	body := binary.AppendUvarint(nil, uint64(len(login)))
	body = append(body, login...)

	var wire bytes.Buffer
	wire.WriteByte(0)
	writer, err := flate.NewWriter(&wire, flate.DefaultCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return wire.Bytes()
}

func TestLoginDiagnosisKeepsCertificateOutOfLogs(t *testing.T) {
	secret := "fixture.certificate.value"
	diagnosis := DescribeBatch(buildLoginWire(t, secret), true)
	if !strings.Contains(diagnosis, "chain=true，Certificate=false") {
		t.Fatalf("unexpected diagnosis: %s", diagnosis)
	}
	if strings.Contains(diagnosis, secret) {
		t.Fatal("certificate content leaked into the diagnosis")
	}
}

func TestMalformedBatchesAreSafeToDescribe(t *testing.T) {
	wire := []byte{0xff, 5, 2, 0, 0, 0, 1}
	if diagnosis := DescribeBatch(wire, true); !strings.Contains(diagnosis, "PlayStatus=1") {
		t.Fatalf("missing PlayStatus diagnosis: %s", diagnosis)
	}
	for _, malformed := range [][]byte{nil, {0}, {0xff, 255}, {0xff, 100, 1}, {0, 0, 1}} {
		DescribeBatch(malformed, true)
	}
}

func TestObserverCountsFirstPacketAndEmitsTrace(t *testing.T) {
	stream := relaytest.New()
	var trace []string
	observer := &Observer{
		Stream: stream,
		Label:  "fixture",
		Trace:  func(line string) { trace = append(trace, line) },
	}
	if !stream.Send([]byte{0xff, 1}) {
		t.Fatal("stream closed before the test packet")
	}
	if _, err := observer.ReadPacket(); err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if got := observer.PacketCount(); got != 1 {
		t.Fatalf("PacketCount = %d, want 1", got)
	}
	if len(trace) != 1 {
		t.Fatalf("trace lines = %d, want 1", len(trace))
	}
}
