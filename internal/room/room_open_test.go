package room

import (
	"context"
	"testing"
	"time"

	"github.com/emuyx/cubedock/internal/auth"
	"github.com/emuyx/cubedock/internal/config"
)

func TestOpenFailureDoesNotPanic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := Open(ctx, config.DefaultProfile(), auth.RoomInfo{
		RaknetAddress:    "127.0.0.1:1",
		SignalingAddress: "127.0.0.1:2",
		RaknetRand:       make([]byte, 16),
		RaknetAESRand:    make([]byte, 16),
		SignalingSeed:    make([]byte, 16),
		SignalingTicket:  make([]byte, 16),
		EncryptKey:       make([]byte, 32),
		DecryptKey:       make([]byte, 32),
		ProtocolID:       43,
	}, nil)
	if err == nil {
		t.Fatal("expected connection failure")
	}
}
