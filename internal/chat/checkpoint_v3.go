package chat

import "time"

type v3Checkpoint struct {
	done chan struct{}
	err  error
}

// Concurrent slots share an fsync/checkpoint. Each waiter still checks its
// write version, so a chunk arriving after the snapshot is never ACKed early.
func commitV3Checkpoint(transfer *incomingFile, version uint64) error {
	for {
		transfer.v3Mu.Lock()
		durable := transfer.v3DurableVersion >= version
		transfer.v3Mu.Unlock()
		if durable {
			return nil
		}
		transfer.v3CheckpointMu.Lock()
		pending := transfer.v3Checkpoint
		leader := pending == nil
		if leader {
			pending = &v3Checkpoint{done: make(chan struct{})}
			transfer.v3Checkpoint = pending
		}
		transfer.v3CheckpointMu.Unlock()
		if leader {
			// Bound added small-file latency while allowing other slots to join the
			// same durable write batch. No goroutine remains after the file ends.
			time.Sleep(500 * time.Microsecond)
			err := persistIncomingResume(transfer)
			transfer.v3CheckpointMu.Lock()
			pending.err = err
			transfer.v3Checkpoint = nil
			close(pending.done)
			transfer.v3CheckpointMu.Unlock()
		} else {
			<-pending.done
		}
		if pending.err != nil {
			return pending.err
		}
	}
}
