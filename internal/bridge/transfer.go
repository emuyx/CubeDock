package bridge

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/Happy2018new/nemc-tan-lobby-solver/minecraft/protocol"
	"github.com/Happy2018new/nemc-tan-lobby-solver/minecraft/protocol/packet"

	"github.com/emuyx/cubedock/internal/gamebatch"
)

// BuildTransferBatch encodes a server-to-client Bedrock Transfer packet. The
// returned bytes are a complete uncompressed game batch as carried by
// NetherNet (without the RakNet 0xfe marker).
func BuildTransferBatch(address string, port uint16) ([]byte, error) {
	if address == "" {
		return nil, errors.New("转服地址为空")
	}
	if port == 0 {
		return nil, errors.New("转服端口无效")
	}
	var payload bytes.Buffer
	writer := protocol.NewWriter(&payload, 0)
	transfer := packet.Transfer{Address: address, Port: port, ReloadWorld: false}
	transfer.Marshal(writer)

	packetBytes := binary.AppendUvarint(nil, uint64(packet.IDTransfer))
	packetBytes = append(packetBytes, payload.Bytes()...)
	return append([]byte{0xff}, gamebatch.Join([][]byte{packetBytes})...), nil
}
