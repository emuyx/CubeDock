package gamebatch

import (
	"bytes"
	"compress/flate"
	"testing"
)

func compress(t *testing.T, plain []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	buffer.WriteByte(0)
	writer, err := flate.NewWriter(&buffer, flate.DefaultCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writer.Write(plain); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestDecodeUncompressedCompressedAndLimit(t *testing.T) {
	plain := bytes.Repeat([]byte{0x2a}, 4096)
	for name, wire := range map[string][]byte{
		"uncompressed": append([]byte{0xff}, plain...),
		"deflate":      compress(t, plain),
	} {
		decoded, err := Decode(wire)
		if err != nil || !bytes.Equal(decoded, plain) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := Decode([]byte{7, 1, 2}); err == nil {
		t.Fatal("unknown compression accepted")
	}
	if _, err := Decode(nil); err == nil {
		t.Fatal("empty batch accepted")
	}
	if _, err := Decode(append([]byte{0xff}, bytes.Repeat([]byte{1}, MaxExpanded+1)...)); err == nil {
		t.Fatal("oversized uncompressed batch accepted")
	}
	if _, err := Decode(compress(t, bytes.Repeat([]byte{1}, MaxExpanded+1))); err == nil {
		t.Fatal("oversized compressed batch accepted")
	}
}

func TestSplitJoinRoundTrip(t *testing.T) {
	packets := [][]byte{{1}, {2, 3, 4}, bytes.Repeat([]byte{9}, 300)}
	joined := Join(packets)
	split, err := Split(joined)
	if err != nil {
		t.Fatal(err)
	}
	if len(split) != len(packets) {
		t.Fatalf("packet count = %d", len(split))
	}
	for index := range packets {
		if !bytes.Equal(split[index], packets[index]) {
			t.Fatalf("packet %d changed", index)
		}
	}
	for _, malformed := range [][]byte{{0x80}, {5, 1, 2}, {0xff, 0xff, 0xff, 0xff, 0x0f}} {
		if _, err := Split(malformed); err == nil {
			t.Fatalf("malformed batch accepted: %x", malformed)
		}
	}
}
