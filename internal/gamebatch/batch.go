// Package gamebatch reads and writes the length-prefixed game packet batches
// carried by NetherNet. Payloads are never interpreted here.
package gamebatch

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// MaxExpanded limits the memory a single compressed batch may occupy. Movement
// traffic can pack many terrain packets together, so the limit is generous.
const MaxExpanded = 32 << 20

// Decode expands one wire batch. A leading 0xff marks an uncompressed body;
// 0 marks DEFLATE; other algorithms are not negotiated by Bedrock.
func Decode(wire []byte) ([]byte, error) {
	if len(wire) == 0 {
		return nil, io.ErrUnexpectedEOF
	}
	switch wire[0] {
	case 0xff:
		if len(wire)-1 > MaxExpanded {
			return nil, errors.New("游戏批次超过 32MiB（解压后）")
		}
		return wire[1:], nil
	case 0:
		inflater := flate.NewReader(bytes.NewReader(wire[1:]))
		defer inflater.Close()
		plain, err := io.ReadAll(io.LimitReader(inflater, MaxExpanded+1))
		if len(plain) > MaxExpanded {
			return nil, errors.New("游戏批次超过 32MiB（解压后）")
		}
		return plain, err
	default:
		return nil, fmt.Errorf("暂不支持压缩算法 %d", wire[0])
	}
}

// Split separates the uvarint-length-prefixed packets inside a decoded batch.
// The returned slices alias data.
func Split(data []byte) ([][]byte, error) {
	var packets [][]byte
	for len(data) > 0 {
		size, head := binary.Uvarint(data)
		if head <= 0 || size > uint64(len(data)-head) {
			return nil, errors.New("游戏批次边界无效")
		}
		packets = append(packets, data[head:head+int(size)])
		data = data[head+int(size):]
	}
	return packets, nil
}

// Join builds the uncompressed body of a batch (without the 0xff marker).
func Join(packets [][]byte) []byte {
	size := 0
	for _, packet := range packets {
		size += binary.MaxVarintLen64 + len(packet)
	}
	body := make([]byte, 0, size)
	for _, packet := range packets {
		body = binary.AppendUvarint(body, uint64(len(packet)))
		body = append(body, packet...)
	}
	return body
}
