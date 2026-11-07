package core

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/emuyx/cubedock/internal/bridge"
)

// TransferPlayer sends a Bedrock Transfer packet to one joined guest. This is a
// POC for the transfer connection mode; it deliberately does not tear down the
// room or change the default proxy path.
func (m *Manager) TransferPlayer(target, address string) (PlayerStatus, error) {
	address = strings.TrimSpace(address)
	host, portText, err := net.SplitHostPort(address)
	if err != nil || strings.TrimSpace(host) == "" {
		return PlayerStatus{}, fmt.Errorf("转服地址应为 host:port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return PlayerStatus{}, fmt.Errorf("转服端口无效")
	}
	batch, err := bridge.BuildTransferBatch(host, uint16(port))
	if err != nil {
		return PlayerStatus{}, err
	}
	status, client, err := m.players.transferTarget(target)
	if err != nil {
		return PlayerStatus{}, err
	}
	if _, err := client.Write(batch); err != nil {
		return PlayerStatus{}, fmt.Errorf("发送转服包失败：%w", err)
	}
	m.emit("控制台", fmt.Sprintf("已请求玩家 %s 转服到 %s",
		playerLabel(status.Name), net.JoinHostPort(host, portText)))
	return status, nil
}
