package minecraft

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Happy2018new/nemc-tan-lobby-solver/minecraft/protocol"
	"github.com/Happy2018new/nemc-tan-lobby-solver/minecraft/protocol/packet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/minecraft/resource"
)

// ListenConfig holds settings that may be edited to change behaviour of a Listener.
type ListenConfig struct {
	// ErrorLog is a log.Logger that errors that occur during packet handling of clients are written to. By
	// default, ErrorLog is set to one equal to the global logger.
	ErrorLog *log.Logger

	// AuthenticationDisabled specifies if authentication of players that join is disabled. If set to true, no
	// verification will be done to ensure that the player connecting is authenticated using their XBOX Live
	// account.
	AuthenticationDisabled bool

	// MaximumPlayers is the maximum amount of players accepted in the server. If non-zero, players that
	// attempt to join while the server is full will be kicked during login. If zero, the maximum player count
	// will be dynamically updated each time a player joins, so that an unlimited amount of players is
	// accepted into the server.
	MaximumPlayers int

	// AllowUnknownPackets specifies if connections of this Listener are allowed to send packets not present
	// in the packet pool. If false (by default), such packets lead to the connection being closed immediately.
	// If set to true, the packets will be returned as a packet.Unknown.
	AllowUnknownPackets bool

	// AllowInvalidPackets specifies if invalid packets (either too few bytes or too many bytes) should be
	// allowed. If false (by default), such packets lead to the connection being closed immediately. If true,
	// packets with too many bytes will be returned while packets with too few bytes will be skipped.
	AllowInvalidPackets bool

	// StatusProvider is the ServerStatusProvider of the Listener. When set to nil, the default provider,
	// ListenerStatusProvider, is used as provider.
	StatusProvider ServerStatusProvider

	// AcceptedProtocols is a slice of Protocol accepted by a Listener created with this ListenConfig. The current
	// Protocol is always added to this slice. Clients with a protocol version that is not present in this slice will
	// be disconnected.
	AcceptedProtocols []Protocol
	// Compression is the packet.Compression to use for packets sent over this Conn. If set to nil, the compression
	// will default to packet.flateCompression.
	Compression packet.Compression // TODO: Change this to snappy once Windows crashes are resolved.
	// FlushRate is the rate at which packets sent are flushed. Packets are buffered for a duration up to
	// FlushRate and are compressed/encrypted together to improve compression ratios. The lower this
	// time.Duration, the lower the latency but the less efficient both network and cpu wise.
	// The default FlushRate (when set to 0) is time.Second/20. If FlushRate is set negative, packets
	// will not be flushed automatically. In this case, calling `(*Conn).Flush()` is required after any
	// calls to `(*Conn).Write()` or `(*Conn).WritePacket()` to send the packets over network.
	FlushRate time.Duration

	// ResourcePacks is a slice of resource packs that the listener may hold. Each client will be asked to
	// download these resource packs upon joining.
	// Use Listener.AddResourcePack() to add a resource pack and Listener.RemoveResourcePack() to remove a resource pack
	// after having called ListenConfig.Listen(). Note that these methods will not update resource packs for active connections.
	ResourcePacks []*resource.Pack
	// Biomes contains information about all biomes that the server has registered, which the client can use
	// to render the world more effectively. If these are nil, the default biome definitions will be used.
	Biomes map[string]any
	// TexturePacksRequired specifies if clients that join must accept the texture pack in order for them to
	// be able to join the server. If they don't accept, they can only leave the server.
	TexturePacksRequired bool

	// PacketFunc is called whenever a packet is read from or written to a connection returned when using
