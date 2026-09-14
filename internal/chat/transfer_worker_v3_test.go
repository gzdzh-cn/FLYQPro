package chat

import (
	"context"
	"testing"
)

func TestSplitChunkTasksV3(t *testing.T) {
	got := SplitChunkTasksV3("x", []ByteRange{{0, 5}, {10, 12}}, 2)
	if len(got) != 4 || got[3].Offset != 10 {
		t.Fatalf("%v", got)
	}
}
func TestChunkWorkerPoolV3Retries(t *testing.T) {
	pool := NewPeerPool("p", 2)
	workers := NewChunkWorkerPoolV3(pool, 2)
	calls := 0
	res := workers.Run(context.Background(), []ChunkTaskV3{{TransferID: "x", Sequence: 1, Length: 1}}, func(_ context.Context, _ *PoolSlot, _ ChunkTaskV3) error {
		calls++
		if calls == 1 {
			return context.DeadlineExceeded
		}
		return nil
	})
	if len(res) != 1 || res[0].Err != nil || calls != 2 {
		t.Fatalf("calls=%d res=%+v", calls, res)
	}
}

func TestPartitionV3RangesSkipsCompletedGaps(t *testing.T) {
	parts := partitionV3Ranges(100, []ByteRange{{Start: 10, End: 20}, {Start: 70, End: 80}}, 3)
	var total int64
	for _, part := range parts {
		for _, r := range part {
			total += r.End - r.Start
			if r.Start < 20 && r.End > 10 || r.Start < 80 && r.End > 70 {
				t.Fatalf("range includes completed bytes: %+v", r)
			}
		}
	}
	if total != 80 {
		t.Fatalf("missing bytes=%d", total)
	}
}
