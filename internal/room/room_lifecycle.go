package room

import (
	"net"
	"strconv"

	"github.com/Happy2018new/nemc-tan-lobby-solver/core/nethernet"
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
