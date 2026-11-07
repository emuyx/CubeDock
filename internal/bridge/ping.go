package bridge

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Happy2018new/nemc-tan-lobby-solver/core/raknet"
)

const pingTimeout = 3 * time.Second

type pongInfo struct {
	edition string
	version string
}

// PingServer probes one NetEase RakNet endpoint and returns the Bedrock
// version string advertised in its unconnected pong.
func PingServer(ctx context.Context, address string) (string, error) {
	raw, err := probeServer(ctx, address)
	if err != nil {
		return "", err
	}
	info, err := decodePong(raw)
	if err != nil {
		return "", err
	}
	return info.version, nil
}

func probeServer(ctx context.Context, address string) ([]byte, error) {
	child, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	return raknet.PingContext(child, address)
}

func decodePong(raw []byte) (pongInfo, error) {
	fields := strings.Split(string(raw), ";")
	if len(fields) < 4 {
		return pongInfo{}, errors.New("目标不是 Bedrock 服务器")
	}
	info := pongInfo{edition: fields[0], version: strings.TrimSpace(fields[3])}
	if !strings.EqualFold(info.edition, "MCPE") {
		return pongInfo{}, errors.New("目标不是 Bedrock 服务器")
	}
	if info.version == "" {
		return pongInfo{}, errors.New("目标服务器未返回版本号")
	}
	return info, nil
}
