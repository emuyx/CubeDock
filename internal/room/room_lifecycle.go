package room

import (
	"fmt"
	"net"
	"strconv"

	"github.com/Happy2018new/nemc-tan-lobby-solver/core/nethernet"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/packet"
)

// Accept waits for the next guest's NetherNet connection.
func (h *Host) Accept() (net.Conn, error) {
	conn, err := h.listener.Accept()
	if err == nil && conn != nil {
		if addr, ok := conn.RemoteAddr().(*nethernet.Addr); ok {
			h.ready.stopNetwork(strconv.FormatUint(addr.NetworkID, 10))
		}
	}
	return conn, err
}

// PlayerIDFor returns the TanLobby player ID assigned to a NetherNet guest.
func (h *Host) PlayerIDFor(conn net.Conn) uint32 {
	if conn == nil {
		return 0
	}
	addr, ok := conn.RemoteAddr().(*nethernet.Addr)
	if !ok {
		return 0
	}
	h.guestMu.Lock()
	defer h.guestMu.Unlock()
	return h.guestPlayers[addr.NetworkID]
}

func (h *Host) removeGuest(playerID uint32) {
	h.guestMu.Lock()
	for networkID, id := range h.guestPlayers {
		if id == playerID {
			delete(h.guestPlayers, networkID)
			break
		}
	}
	h.guestMu.Unlock()
}

// KickPlayer asks the TanLobby control node to remove one guest from the room.
// The upstream server owns the actual disconnect; the caller should not close
// the guest's NetherNet connection itself unless this call fails.
func (h *Host) KickPlayer(playerID uint32) error {
	if playerID == 0 {
		return fmt.Errorf("房间玩家 ID 无效")
	}
	if err := h.send(&packet.TanKickOutRequest{PlayerID: playerID}); err != nil {
		return fmt.Errorf("发送房间踢出请求失败: %w", err)
	}
	if h.trace != nil {
		h.trace(fmt.Sprintf("已发送房间踢出请求：玩家 %d", playerID))
	}
	return nil
}

// Done closes when the control connection ends.
func (h *Host) Done() <-chan struct{} { return h.done }

// Err reports why the room ended, if it failed.
func (h *Host) Err() error {
	h.errMu.Lock()
	defer h.errMu.Unlock()
	return h.err
}

// Close tears down the room. Disconnecting the owner's RakNet session removes
// it from NetEase, so no shutdown packet is invented.
func (h *Host) Close() {
	h.closeOnce.Do(func() {
		close(h.done)
		if h.control != nil {
			_ = h.control.Close()
		}
		if h.listener != nil {
			_ = h.listener.Close()
			return
		}
		if h.signaling != nil {
			h.signaling.Close(net.ErrClosed)
		}
	})
}
