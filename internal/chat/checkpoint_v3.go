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
			err := persistIncomingV3Checkpoint(transfer)
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

// persistIncomingV3Checkpoint snapshots the written ranges before doing any
// blocking storage work. Chunks arriving after the snapshot are intentionally
// left for the next checkpoint and cannot be acknowledged by this one.
func persistIncomingV3Checkpoint(transfer *incomingFile) error {
	if transfer == nil || transfer.file == nil {
		return nil
	}
	transfer.v3Mu.Lock()
	if transfer.v3DurableVersion >= transfer.v3WrittenVersion {
		transfer.v3Mu.Unlock()
		return nil
	}
	snapshotVersion := transfer.v3WrittenVersion
	ranges := append([]ByteRange(nil), transfer.v3Ranges...)
	transfer.v3Mu.Unlock()

	// Do not hold v3Mu while the filesystem or SQLite/sidecar checkpoint runs.
	// This lets other slots continue writing the next batch into memory.
	if err := syncTransferFile(transfer.file); err != nil {
		return err
	}

	state := transferResumeState{}
	transfer.resumeMu.Lock()
	state = transfer.resumeState
	state.CompletedRanges = make([]TransferRange, 0, len(ranges))
	var covered int64
	for _, r := range ranges {
		if r.End <= r.Start {
			continue
		}
		state.CompletedRanges = append(state.CompletedRanges, TransferRange{Offset: r.Start, Length: r.End - r.Start})
		covered += r.End - r.Start
	}
	state.TransferMode = v3TransferMode
	if state.AttachmentID != "" {
		state.CheckpointSeq++
		if err := saveTransferResumeState(state); err != nil {
			transfer.resumeMu.Unlock()
			return err
		}
	}
	transfer.resumeState = state
	transfer.resumeMu.Unlock()

	transfer.v3Mu.Lock()
	if snapshotVersion > transfer.v3DurableVersion {
		transfer.v3DurableVersion = snapshotVersion
	}
	if covered > transfer.durableBytes {
		transfer.durableBytes = covered
	}
	transfer.v3Mu.Unlock()
	return nil
}
