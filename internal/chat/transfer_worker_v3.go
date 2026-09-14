package chat

import (
	"context"
	"errors"
	"sync"
)

type ChunkTaskV3 struct {
	TransferID string
	Sequence   uint64
	Offset     int64
	Length     int64
	Attempts   int
}
type ChunkResultV3 struct {
	Task ChunkTaskV3
	Err  error
}

func SplitChunkTasksV3(transferID string, ranges []ByteRange, chunkSize int64) []ChunkTaskV3 {
	if chunkSize <= 0 {
		chunkSize = 512 << 10
	}
	var out []ChunkTaskV3
	var seq uint64
	for _, r := range MergeRanges(ranges) {
		for pos := r.Start; pos < r.End; {
			end := pos + chunkSize
			if end > r.End {
				end = r.End
			}
			out = append(out, ChunkTaskV3{TransferID: transferID, Sequence: seq, Offset: pos, Length: end - pos})
			seq++
			pos = end
		}
	}
	return out
}

type ChunkWorkerPoolV3 struct {
	slots       *PeerPool
	maxAttempts int
	mu          sync.Mutex
	pending     map[uint64]ChunkTaskV3
}

func NewChunkWorkerPoolV3(slots *PeerPool, maxAttempts int) *ChunkWorkerPoolV3 {
	if maxAttempts < 1 {
		maxAttempts = 3
	}
	return &ChunkWorkerPoolV3{slots: slots, maxAttempts: maxAttempts, pending: make(map[uint64]ChunkTaskV3)}
}
func (p *ChunkWorkerPoolV3) Run(ctx context.Context, tasks []ChunkTaskV3, send func(context.Context, *PoolSlot, ChunkTaskV3) error) []ChunkResultV3 {
	for _, t := range tasks {
		p.mu.Lock()
		p.pending[t.Sequence] = t
		p.mu.Unlock()
	}
	results := make([]ChunkResultV3, 0, len(tasks))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, initial := range tasks {
		task := initial
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			for task.Attempts = 0; task.Attempts < p.maxAttempts; task.Attempts++ {
				slot, e := p.slots.Acquire(ctx, task.TransferID)
				if e != nil {
					err = e
					break
				}
				err = send(ctx, slot, task)
				if err == nil {
					p.slots.Release(slot)
					break
				}
				slot.Kill()
				p.slots.ReplaceDead()
			}
			p.mu.Lock()
			delete(p.pending, task.Sequence)
			p.mu.Unlock()
			mu.Lock()
			results = append(results, ChunkResultV3{Task: task, Err: err})
			mu.Unlock()
		}()
	}
	wg.Wait()
	return results
}
func RetryableChunkError(err error) bool {
	return err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}
