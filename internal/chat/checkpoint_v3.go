package chat

import "time"

type v3Checkpoint struct {
	done chan struct{}
	err  error
}

// Concurrent slots share an fsync/checkpoint. Each waiter still checks its
// write version, so a chunk arriving after the snapshot is never ACKed early.
func commitV3Checkpoint(engine *Engine, transfer *incomingFile, version uint64) error {
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
			// Give parallel slots a short coalescing window. The range/version
			// snapshot is still taken inside persistIncomingV3Checkpoint, so data
			// arriving after it cannot be acknowledged by this batch.
			time.Sleep(2 * time.Millisecond)
			err := persistIncomingV3Checkpoint(engine, transfer)
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
func persistIncomingV3Checkpoint(engine *Engine, transfer *incomingFile) error {
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
	syncStarted := time.Now()
	if err := syncTransferFile(transfer.file); err != nil {
		return err
	}
	syncDuration := time.Since(syncStarted)

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
	if engine != nil {
		elapsedMs, metricGeneration := engine.transferMetricSnapshot(transfer.attachmentID, "receive")
		if elapsedMs > state.ElapsedMs {
			state.ElapsedMs = elapsedMs
		}
		if metricGeneration > 0 {
			state.MetricGeneration = metricGeneration
		}
		if covered > state.MetricLastBytes {
			state.MetricLastBytes = covered
		}
	}
	if state.AttachmentID != "" {
		state.CheckpointSeq++
		checkpointSnapshot := snapshotFromResume(state)
		checkpointSnapshot.Direction = "receive"
		checkpointSnapshot.Phase = "checkpoint_persist"
		checkpointSnapshot.State = TransferActive
		checkpointSnapshot.Transferred = covered
		checkpointSnapshot.DurableBytes = covered
		checkpointSnapshot.Percent = transferProgressPercent(covered, state.FileSize, checkpointSnapshot.Phase)
		checkpointSnapshot.UpdatedAt = time.Now().UTC()
		transfer.v3Mu.Lock()
		checkpointSnapshot.MetricSeq = transfer.v3MetricSeq
		checkpointSnapshot.Speed = transfer.v3LastSpeed
		checkpointSnapshot.AverageSpeed = transfer.v3AverageSpeed
		checkpointSnapshot.PeakSpeed = transfer.v3PeakSpeed
		checkpointSnapshot.DiskWriteMs = syncDuration.Milliseconds()
		checkpointSnapshot.AckLatencyMs = transfer.v3LastAckLatencyMs
		checkpointSnapshot.ReceiverWriteMs = transfer.v3ReceiverWriteMs
		checkpointSnapshot.DurabilitySyncMs = transfer.v3DurabilitySyncMs + syncDuration.Milliseconds()
		checkpointSnapshot.ResumePersistMs = transfer.v3ResumePersistMs
		checkpointSnapshot.DataTransferMs = state.ElapsedMs
		checkpointSnapshot.TransferMode = v3TransferMode
		checkpointSnapshot.Transport = "TLS13/TCP-v3"
		checkpointSnapshot.StreamCount = len(transfer.v3Streams)
		checkpointSnapshot.ActiveStreams = checkpointSnapshot.StreamCount
		transfer.v3Mu.Unlock()
		persistStarted := time.Now()
		if err := saveTransferResumeCheckpoint(state, checkpointSnapshot); err != nil {
			transfer.resumeMu.Unlock()
			return err
		}
		transfer.v3Mu.Lock()
		transfer.v3DurabilitySyncMs += syncDuration.Milliseconds()
		transfer.v3ResumePersistMs += time.Since(persistStarted).Milliseconds()
		transfer.v3Mu.Unlock()
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
	if engine != nil {
		engine.recordTransferStage(transfer.attachmentID, transfer.senderID, "checkpoint", -1, "", time.Since(syncStarted), covered, false, "")
	}
	return nil
}
