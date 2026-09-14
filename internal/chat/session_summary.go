package chat

import (
	"fmt"
	"sort"
	"time"
)

// SessionSummary is a read-only view of one persistent peer pool. It is
// intentionally transport-neutral so the UI and diagnostics do not need to
// know whether the pool is backed by TLS/TCP or a future QUIC adapter.
type SessionSummary struct {
	PeerDeviceID string    `json:"peerDeviceId"`
	SessionID    string    `json:"sessionId"`
	Generation   uint64    `json:"generation"`
	SlotCount    int       `json:"slotCount"`
	ActiveSlots  int       `json:"activeSlots"`
	DeadSlots    int       `json:"deadSlots"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (p *PeerPool) Summary() SessionSummary {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := SessionSummary{
		PeerDeviceID: p.peerID,
		SessionID:    fmt.Sprintf("%x", p.sessionID),
		Generation:   p.generation,
		SlotCount:    len(p.slots),
		UpdatedAt:    time.Now().UTC(),
	}
	for _, slot := range p.slots {
		state, _ := slot.Snapshot()
		switch state {
		case SlotReserved, SlotTransferring, SlotDraining, SlotReconnecting:
			result.ActiveSlots++
		case SlotDead:
			result.DeadSlots++
		}
	}
	return result
}

// ListSessionSummaries exposes only current pool metadata; sockets and task
// internals remain private to the transfer engine.
func (e *Engine) ListSessionSummaries() []SessionSummary {
	e.mu.RLock()
	pools := make([]*PeerPool, 0, len(e.peerPools))
	for _, pool := range e.peerPools {
		if pool != nil {
			pools = append(pools, pool)
		}
	}
	e.mu.RUnlock()
	result := make([]SessionSummary, 0, len(pools))
	for _, pool := range pools {
		result = append(result, pool.Summary())
	}
	sort.Slice(result, func(i, j int) bool { return result[i].PeerDeviceID < result[j].PeerDeviceID })
	return result
}

func (e *Engine) ActiveTransferCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.incoming) + len(e.outgoing) + len(e.preparing) + len(e.sharedTransfers)
}
