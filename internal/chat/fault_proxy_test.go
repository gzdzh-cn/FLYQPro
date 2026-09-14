package chat

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// scriptedFaultConn is the deterministic transport fixture used by recovery
// tests. It can delay IO, disconnect after an exact byte count, and mutate one
// write without adding production-only branches to the transfer path.
type scriptedFaultConn struct {
	net.Conn
	mu              sync.Mutex
	written         int
	disconnectAfter int
	delay           time.Duration
	readDelay       time.Duration
	dropWrites      int
	duplicateWrites int
	mutateWrite     func([]byte) []byte
}

func (c *scriptedFaultConn) Read(payload []byte) (int, error) {
	if c.readDelay > 0 {
		time.Sleep(c.readDelay)
	}
	return c.Conn.Read(payload)
}

func (c *scriptedFaultConn) Write(payload []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	if c.disconnectAfter >= 0 && c.written >= c.disconnectAfter {
		return 0, io.ErrClosedPipe
	}
	if c.dropWrites > 0 {
		c.dropWrites--
		return len(payload), nil
	}
	data := append([]byte(nil), payload...)
	if c.mutateWrite != nil {
		data = c.mutateWrite(data)
		c.mutateWrite = nil
	}
	if c.disconnectAfter >= 0 && c.written+len(data) > c.disconnectAfter {
		data = data[:c.disconnectAfter-c.written]
	}
	n, err := c.Conn.Write(data)
	if err == nil {
		for i := 0; i < c.duplicateWrites; i++ {
			if _, duplicateErr := c.Conn.Write(data); duplicateErr != nil {
				err = duplicateErr
				break
			}
		}
	}
	c.written += n
	if err == nil && n < len(payload) {
		err = io.ErrClosedPipe
	}
	return n, err
}

func TestScriptedFaultConnSupportsDelayDropAndDuplicateWrites(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	fault := &scriptedFaultConn{Conn: left, disconnectAfter: -1, dropWrites: 1, readDelay: time.Millisecond}
	if n, err := fault.Write([]byte("drop")); err != nil || n != 4 {
		t.Fatalf("dropped write should be acknowledged to caller: n=%d err=%v", n, err)
	}
	left2, right2 := net.Pipe()
	defer left2.Close()
	defer right2.Close()
	fault = &scriptedFaultConn{Conn: left2, disconnectAfter: -1, duplicateWrites: 1}
	received := make(chan []byte, 1)
	go func() {
		buffer := make([]byte, 8)
		_, _ = io.ReadFull(right2, buffer)
		received <- buffer
	}()
	if n, err := fault.Write([]byte("echo")); err != nil || n != 4 {
		t.Fatalf("duplicate write failed: n=%d err=%v", n, err)
	}
	select {
	case got := <-received:
		if string(got) != "echoecho" {
			t.Fatalf("unexpected duplicated payload: %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for duplicated payload")
	}
}

func TestScriptedFaultConnDisconnectIsRepeatable(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	fault := &scriptedFaultConn{Conn: left, disconnectAfter: 4}
	done := make(chan error, 1)
	go func() {
		buffer := make([]byte, 4)
		_, err := io.ReadFull(right, buffer)
		done <- err
	}()
	if n, err := fault.Write([]byte("12345678")); n != 4 || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("unexpected injected disconnect: n=%d err=%v", n, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n, err := fault.Write([]byte("x")); n != 0 || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("disconnect did not remain active: n=%d err=%v", n, err)
	}
}

func TestScriptedFaultConnMutatesExactlyOneWrite(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	fault := &scriptedFaultConn{Conn: left, disconnectAfter: -1, mutateWrite: func(data []byte) []byte {
		data[0] ^= 0xff
		return data
	}}
	received := make(chan []byte, 1)
	go func() {
		buffer := make([]byte, 2)
		_, _ = io.ReadFull(right, buffer)
		received <- buffer
	}()
	if _, err := fault.Write([]byte{1, 2}); err != nil {
		t.Fatal(err)
	}
	if got := <-received; got[0] != 0xfe || got[1] != 2 {
		t.Fatalf("unexpected mutation: %v", got)
	}
}
