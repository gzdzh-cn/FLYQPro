package chat

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

type PoolSlotState uint8

const (
	SlotIdle PoolSlotState = iota
	SlotReserved
	SlotTransferring
	SlotDraining
	SlotDead
	SlotReconnecting
)

type PoolSlot struct {
	ID                    int
	Generation            uint64
	State                 PoolSlotState
	CurrentTask           string
	LastProgress          time.Time
	BytesSent, BytesAcked int64
	conn                  net.Conn
	readCh                chan poolSlotReadResult
	readCancel            context.CancelFunc
	lastIOAt              time.Time
	lastAckAt             time.Time
	inFlightBytes         int64
	currentTransferID     string
	leaseToken            uint64
	lastPingAt            time.Time
	terminal              bool
	mu                    sync.Mutex
	ioMu                  sync.Mutex
}

type poolSlotReadResult struct {
	frame BinaryFrameV3
	err   error
}

func (s *PoolSlot) Reserve(task string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != SlotIdle {
		return errors.New("slot unavailable")
	}
	s.State = SlotReserved
	s.CurrentTask = task
	s.currentTransferID = task
	s.leaseToken++
	s.terminal = false
	s.LastProgress = time.Now()
	s.lastIOAt = s.LastProgress
	s.lastAckAt = time.Time{}
	s.lastPingAt = time.Time{}
	s.inFlightBytes = 0
	return nil
}
func (s *PoolSlot) Start() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != SlotReserved {
		return false
	}
	s.State = SlotTransferring
	return true
}
func (s *PoolSlot) Progress(sent, acked int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.BytesSent += sent
	s.BytesAcked += acked
	s.inFlightBytes -= acked
	if s.inFlightBytes < 0 {
		s.inFlightBytes = 0
	}
	s.LastProgress = time.Now()
	s.lastIOAt = s.LastProgress
	if acked > 0 {
		s.lastAckAt = s.LastProgress
	}
}

// TouchIO records transport activity independently from durable progress. A
// slow receiver can therefore keep a slot alive while it is still reading or
// writing a frame but has not emitted the batch ACK yet.
func (s *PoolSlot) TouchIO(direction string) {
	s.mu.Lock()
	s.lastIOAt = time.Now()
	s.lastPingAt = time.Time{}
	s.mu.Unlock()
}

func (s *PoolSlot) MarkSent(bytes int64) {
	s.mu.Lock()
	s.BytesSent += bytes
	s.inFlightBytes += bytes
	s.lastIOAt = time.Now()
	s.lastPingAt = time.Time{}
	s.mu.Unlock()
}

func (s *PoolSlot) MarkAck(bytes int64) {
	s.mu.Lock()
	s.BytesAcked += bytes
	s.inFlightBytes -= bytes
	if s.inFlightBytes < 0 {
		s.inFlightBytes = 0
	}
	now := time.Now()
	s.lastAckAt = now
	s.lastIOAt = now
	s.lastPingAt = time.Time{}
	s.LastProgress = now
	s.mu.Unlock()
}

func (s *PoolSlot) IsOwner(transferID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.terminal && s.currentTransferID == transferID && s.State != SlotDead
}

func (s *PoolSlot) LeaseToken() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.leaseToken
}

func (s *PoolSlot) IsLeaseOwner(transferID string, leaseToken uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.terminal && s.currentTransferID == transferID && s.leaseToken == leaseToken && s.State != SlotDead
}

func (s *PoolSlot) TransferID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentTransferID
}
func (s *PoolSlot) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != SlotDead {
		s.State = SlotIdle
	}
	s.CurrentTask = ""
	s.currentTransferID = ""
	s.terminal = false
	s.LastProgress = time.Now()
	s.lastIOAt = s.LastProgress
	s.inFlightBytes = 0
}
func (s *PoolSlot) Connection() net.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn
}
func (s *PoolSlot) SetConnection(conn net.Conn) {
	s.mu.Lock()
	previous := s.conn
	previousCancel := s.readCancel
	s.conn = conn
	s.readCh = nil
	s.readCancel = nil
	var readCtx context.Context
	if conn != nil {
		s.readCh = make(chan poolSlotReadResult, 64)
		readCtx, s.readCancel = context.WithCancel(context.Background())
	}
	readCh := s.readCh
	s.mu.Unlock()
	if previousCancel != nil {
		previousCancel()
	}
	if previous != nil && previous != conn {
		_ = previous.Close()
	}
	if conn != nil {
		go s.drainReplies(readCtx, conn, readCh)
	}
}

// drainReplies is the sole reader for a leased TLS slot. It remains alive
// while the connection is idle so late Pong/ACK frames cannot be consumed by
// the next transfer's synchronous read path.
func (s *PoolSlot) drainReplies(ctx context.Context, conn net.Conn, output chan poolSlotReadResult) {
	defer close(output)
	for {
		frame, err := ReadBinaryFrameV3(conn, 64*1024)
		if err != nil {
			select {
			case output <- poolSlotReadResult{err: err}:
			default:
			}
			return
		}
		s.TouchIO("read")
		select {
		case output <- poolSlotReadResult{frame: frame}:
		case <-ctx.Done():
			return
		}
	}
}

func (s *PoolSlot) ReadFrame(ctx context.Context) (BinaryFrameV3, error) {
	s.mu.Lock()
	readCh := s.readCh
	valid := s.conn != nil
	s.mu.Unlock()
	if !valid || readCh == nil {
		return BinaryFrameV3{}, net.ErrClosed
	}
	select {
	case <-ctx.Done():
		return BinaryFrameV3{}, ctx.Err()
	case result, ok := <-readCh:
		if !ok {
			return BinaryFrameV3{}, io.ErrUnexpectedEOF
		}
		return result.frame, result.err
	}
}

func (s *PoolSlot) ReadFrameForLease(ctx context.Context, transferID string, leaseToken uint64) (BinaryFrameV3, error) {
	if !s.IsLeaseOwner(transferID, leaseToken) {
		return BinaryFrameV3{}, net.ErrClosed
	}
	frame, err := s.ReadFrame(ctx)
	if err == nil && !s.IsLeaseOwner(transferID, leaseToken) {
		return BinaryFrameV3{}, net.ErrClosed
	}
	return frame, err
}
func (s *PoolSlot) CloseConnection() {
	s.mu.Lock()
	conn := s.conn
	cancel := s.readCancel
	s.conn = nil
	s.readCh = nil
	s.readCancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close()
	}
}
func (s *PoolSlot) Kill() {
	s.mu.Lock()
	conn := s.conn
	cancel := s.readCancel
	s.conn = nil
	s.readCh = nil
	s.readCancel = nil
	s.State = SlotDead
	s.CurrentTask = ""
	s.currentTransferID = ""
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close()
	}
}

// Cancel is a terminal transition for the current transfer. It is separate
// from Kill so callers can distinguish user cancellation from a dead socket.
func (s *PoolSlot) Cancel() {
	s.mu.Lock()
	conn := s.conn
	cancel := s.readCancel
	s.conn = nil
	s.readCh = nil
	s.readCancel = nil
	s.State = SlotDead
	s.CurrentTask = ""
	s.currentTransferID = ""
	s.terminal = true
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if conn != nil {
		_ = conn.Close()
	}
}

func (s *PoolSlot) ProbeNeeded(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return (s.State == SlotTransferring || s.State == SlotReserved) && s.inFlightBytes > 0 &&
		now.Sub(s.lastIOAt) >= 2*time.Second && s.lastPingAt.IsZero()
}

func (s *PoolSlot) MarkProbe(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastPingAt.IsZero() {
		s.lastPingAt = now
		s.lastIOAt = now
		return true
	}
	return false
}

func (s *PoolSlot) ProbeExpired(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	// A durable ACK can legitimately be delayed by a several-second file
	// sync/checkpoint. Only expire a probe after a long quiet period; any read,
	// write, ACK, or Pong clears lastPingAt through TouchIO/MarkAck.
	const probeTimeout = 10 * time.Second
	return (s.State == SlotTransferring || s.State == SlotReserved) && !s.lastPingAt.IsZero() && now.Sub(s.lastPingAt) >= probeTimeout
}

func (s *PoolSlot) WriteFrame(frame BinaryFrameV3) error {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	s.mu.Lock()
	conn := s.conn
	valid := !s.terminal && (s.State == SlotReserved || s.State == SlotTransferring || s.State == SlotDraining)
	s.mu.Unlock()
	if !valid || conn == nil {
		return net.ErrClosed
	}
	if err := writeV3Frame(conn, frame); err != nil {
		return err
	}
	if frame.Type == FramePoolPing {
		// The probe write is transport activity, but it must not clear the
		// pending probe timestamp or the watchdog can never detect a dead peer.
		s.mu.Lock()
		s.lastIOAt = time.Now()
		s.mu.Unlock()
	} else {
		s.TouchIO("write")
	}
	return nil
}

func (s *PoolSlot) WriteFrameForLease(transferID string, leaseToken uint64, frame BinaryFrameV3) error {
	if !s.IsLeaseOwner(transferID, leaseToken) {
		return net.ErrClosed
	}
	return s.WriteFrame(frame)
}
func (s *PoolSlot) Stalled(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return (s.State == SlotTransferring || s.State == SlotReserved) && s.inFlightBytes > 0 && now.Sub(s.lastIOAt) >= 2*time.Second
}

func (s *PoolSlot) Snapshot() (PoolSlotState, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State, s.LastProgress
}
func (s *PoolSlot) Drain() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State == SlotTransferring {
		s.State = SlotDraining
	}
}
