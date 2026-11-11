package netease

import (
	"bytes"
	"testing"
)

func TestPeAuthSignCapturedVector(t *testing.T) {
	message := interopEngineVersion + interopLibraryHash + interopPatchVersion + interopPatchHash +
		interopSignHash + "22178e42-e1ba-4e81-b385-8048100be667"
	sign, err := peAuthSign(message, interopSignSP, interopSignTR)
	if err != nil {
		t.Fatal(err)
	}
	if sign != "Sdy8Dgl3TCEJZ8sokRTm/Q==" {
		t.Fatalf("sign = %q", sign)
	}
}

func TestEnvelopeRoundTrip(t *testing.T) {
	plaintext := bytes.Repeat([]byte(`{"code":0,"message":"ok"}`), 3)
	random := bytes.NewReader(bytes.Repeat([]byte{7}, 33))
	sealed, err := encryptEnvelope(plaintext, random)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := decryptEnvelope(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(opened, plaintext) {
		t.Fatalf("round trip mismatch: %q", opened)
	}
}

func TestEnvelopeRejectsUnknownVersion(t *testing.T) {
	if _, err := decryptEnvelope("00"); err == nil {
		t.Fatal("short envelope accepted")
	}
}
