package nethernet

import (
	"bytes"
	"testing"
)

func newMessageConn() *Conn {
	return &Conn{
		packets: make(chan []byte, 1),
		closed:  make(chan struct{}),
		message: &message{},
	}
}

func TestHandleMessageReassemblesSegments(t *testing.T) {
	conn := newMessageConn()
	if err := conn.handleMessage([]byte{1, 'a'}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-conn.packets:
		t.Fatal("incomplete message was delivered")
	default:
	}
	if err := conn.handleMessage([]byte{0, 'b'}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-conn.packets:
		if !bytes.Equal(got, []byte("ab")) {
			t.Fatalf("message = %q", got)
		}
	default:
		t.Fatal("complete message was not delivered")
	}
}

func TestHandleMessageAcceptsContinuationFlags(t *testing.T) {
	conn := newMessageConn()
	for _, frame := range [][]byte{{1, 'a'}, {1, 'b'}, {0, 'c'}} {
		if err := conn.handleMessage(frame); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case got := <-conn.packets:
		if !bytes.Equal(got, []byte("abc")) {
			t.Fatalf("message = %q", got)
		}
	default:
		t.Fatal("complete message was not delivered")
	}
	if conn.message.data != nil {
		t.Fatal("reassembly state was not reset after delivery")
	}
}

func TestHandleMessageRejectsOversizedMessage(t *testing.T) {
	conn := newMessageConn()
	for i := 0; i < maxReassembledMessage/(1<<20)+1; i++ {
		frame := make([]byte, (1<<20)+1)
		frame[0] = 1
		err := conn.handleMessage(frame)
		if err != nil {
			if conn.message.data != nil {
				t.Fatal("reassembly buffer survived the limit error")
			}
			return
		}
	}
	t.Fatal("oversized message was accepted")
}
