package chat

import (
	"errors"
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
	mu                    sync.Mutex
}

func (s *PoolSlot) Reserve(task string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != SlotIdle {
		return errors.New("slot unavailable")
	}
	s.State = SlotReserved
	s.CurrentTask = task
	s.LastProgress = time.Now()
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
	s.LastProgress = time.Now()
}
func (s *PoolSlot) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != SlotDead {
		s.State = SlotIdle
	}
	s.CurrentTask = ""
	s.LastProgress = time.Now()
}
func (s *PoolSlot) Connection() net.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn
}
func (s *PoolSlot) SetConnection(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conn = conn
}
func (s *PoolSlot) CloseConnection() {
	s.mu.Lock()
	conn := s.conn
	s.conn = nil
	s.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}
func (s *PoolSlot) Kill() {
	s.mu.Lock()
	conn := s.conn
	s.conn = nil
	s.State = SlotDead
	s.CurrentTask = ""
	s.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}
func (s *PoolSlot) Stalled(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return (s.State == SlotTransferring || s.State == SlotReserved) && now.Sub(s.LastProgress) >= 2*time.Second
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
