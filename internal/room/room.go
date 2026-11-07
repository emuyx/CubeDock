// Package room hosts one NetEase TanLobby room. It speaks the room control
// protocol on a RakNet transfer node and exposes joining players as NetherNet
// connections through a WebRTC signaling session.
package room

import (
	"sync"

	"github.com/Happy2018new/nemc-tan-lobby-solver/core/nethernet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/core/raknet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/packet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/service/signaling"
)

// levelID is the map identifier advertised by the room. Guests only use it to
// pick a loading screen; the Bedrock world comes from the upstream server.
const levelID = "CubeDock"

// Host is one live TanLobby room owned by the bot account.
type Host struct {
	ID        uint32
	control   *raknet.Conn
	encoder   *packet.Encoder
	decoder   *packet.Decoder
	listener  *nethernet.Listener
	signaling *signaling.Conn
	networkID uint64
	trace     func(string)
	ready     *readyQueue
	done      chan struct{}
	closeOnce sync.Once
	writeMu   sync.Mutex
	errMu     sync.Mutex
	err       error
}
