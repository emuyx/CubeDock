package nethernet

import (
	"fmt"
	"io"
	"net"
)

// maxReassembledMessage bounds a message split across ReliableDataChannel
// frames. The remote peer controls the segment count, so an unbounded buffer
// lets a guest exhaust the host's memory.
const maxReassembledMessage = 8 << 20

// message represents the structure of remote messages sent in ReliableDataChannel.
type message struct {
	// segments is the continuation flag of the last segment: non-zero while
	// more segments follow, zero on the final one.
	segments uint8
	// data represents the payload of the message.
	data []byte
}

// parseMessage parses the provided data into a message. The first byte indicates the count of segments,
// and the remaining bytes are its payload. If the data is less than 2 bytes, an error is returned.
func parseMessage(b []byte) (*message, error) {
	if len(b) < 2 {
		return nil, io.ErrUnexpectedEOF
	}
	return &message{
		segments: b[0],
		data:     b[1:],
	}, nil
}

// handleMessage handles a message received from a [webrtc.DataChannel] labeled 'ReliableDataChannel'.
// It parses the incoming data into a message using parseMessage and handles the segments, and if all
// segments has been received, it sends the message data to either [Conn.Read] or [Conn.ReadPacket].
func (c *Conn) handleMessage(b []byte) error {
	msg, err := parseMessage(b)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	// The first byte is a continuation flag: 1 while more segments follow, 0
	// on the final one. Only the accumulated size is bounded here.
	if len(c.message.data)+len(msg.data) > maxReassembledMessage {
		c.message = &message{}
		return fmt.Errorf("reassembled message exceeds %d bytes", maxReassembledMessage)
	}
	c.message.segments = msg.segments
	c.message.data = append(c.message.data, msg.data...)
	if c.message.segments > 0 {
		return nil
	}

	data := c.message.data
	c.message = &message{}
	select {
	case c.packets <- data:
	case <-c.closed:
		return net.ErrClosed
	}
	return nil
}
