package chat

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// PeerPool owns reusable transfer slots independently from a file transfer.
// The wire connection is attached by the caller; this layer owns lifecycle and
// scheduling so a later transport can replace TCP without changing workers.
type PeerPool struct {
	mu         sync.Mutex
	peerID     string
	generation uint64
	sessionID  [16]byte
	slots      []*PoolSlot
}

func NewPeerPool(peerID string, limit int) *PeerPool {
	if limit > 8 {
		limit = 8
	}
	if limit < 1 {
		limit = 1
	}
	p := &PeerPool{peerID: peerID, generation: 1}
	_, _ = rand.Read(p.sessionID[:])
	for i := 0; i < limit; i++ {
		p.slots = append(p.slots, &PoolSlot{ID: i, Generation: p.generation, State: SlotIdle, LastProgress: time.Now()})
	}
	return p
}
func (p *PeerPool) SessionID() [16]byte { p.mu.Lock(); defer p.mu.Unlock(); return p.sessionID }
func (p *PeerPool) Generation() uint64  { p.mu.Lock(); defer p.mu.Unlock(); return p.generation }
func (p *PeerPool) Acquire(ctx context.Context, task string) (*PoolSlot, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		p.mu.Lock()
		for _, s := range p.slots {
			if s.Reserve(task) == nil {
				p.mu.Unlock()
				if !s.Start() {
					return nil, fmt.Errorf("slot transition failed")
				}
				return s, nil
			}
		}
		p.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
func (p *PeerPool) Release(s *PoolSlot) {
	if s != nil {
		s.Release()
	}
}

// CloseTransfer interrupts every slot currently owned by one transfer. It is
// intentionally idempotent: a worker may already have returned its slot when
// the UI cancellation arrives.
func (p *PeerPool) CloseTransfer(transferID string) {
	p.mu.Lock()
	slots := append([]*PoolSlot(nil), p.slots...)
	p.mu.Unlock()
	for _, slot := range slots {
		if slot != nil {
			slot.mu.Lock()
			owned := slot.currentTransferID == transferID && !slot.terminal
			slot.mu.Unlock()
			if owned {
				slot.Cancel()
			}
		}
	}
}

// EnsureLimit grows a peer's reusable slot set when a later transfer needs a
// larger pipeline. Existing slots and their generation are preserved.
func (p *PeerPool) EnsureLimit(limit int) {
	if limit > 8 {
		limit = 8
	}
	if limit < 1 {
		limit = 1
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.slots) < limit {
		p.slots = append(p.slots, &PoolSlot{ID: len(p.slots), Generation: p.generation, State: SlotIdle, LastProgress: time.Now()})
	}
}
func (p *PeerPool) ReplaceDead() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, s := range p.slots {
		if state, _ := s.Snapshot(); state == SlotDead {
			// Slot repair does not create a new peer session. Other active
			// slots must keep the same generation as their replacement.
			p.slots[i] = &PoolSlot{ID: s.ID, Generation: p.generation, State: SlotIdle, LastProgress: time.Now()}
		}
	}
}

func (p *PeerPool) CloseIdle(now time.Time, idle time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, slot := range p.slots {
		if state, last := slot.Snapshot(); state == SlotIdle && now.Sub(last) >= idle {
			slot.CloseConnection()
		}
	}
}
func (p *PeerPool) Active() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, s := range p.slots {
		if state, _ := s.Snapshot(); state == SlotTransferring || state == SlotReserved || state == SlotDraining {
			n++
		}
	}
	return n
}

func (p *PeerPool) Watchdog(now time.Time) (stalled int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.slots {
		if s.ProbeNeeded(now) {
			if s.MarkProbe(now) {
				id := binaryTransferID(s.TransferID())
				ping := BinaryFrameV3{Type: FramePoolPing, TransferID: id, StreamID: uint16(s.ID), SessionID: p.sessionID, Generation: s.Generation}
				if conn := s.Connection(); conn != nil {
					_ = conn.SetWriteDeadline(now.Add(500 * time.Millisecond))
				}
				if err := s.WriteFrame(ping); err != nil {
					s.Kill()
					stalled++
				}
				if conn := s.Connection(); conn != nil {
					_ = conn.SetWriteDeadline(time.Time{})
				}
			}
			continue
		}
		if s.ProbeExpired(now) || s.Stalled(now) && !s.ProbeNeeded(now) {
			stalled++
			s.Kill()
		}
	}
	return stalled
}
