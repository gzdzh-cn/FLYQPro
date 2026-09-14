package chat

import (
	"log"
	"time"
)

// transferStageMetrics contains diagnostic timings only. User-facing speed and
// elapsed time continue to come from receiver-durable transfer metrics.
type transferStageMetrics struct {
	ControlDialMs       int64
	OfferWaitMs         int64
	DataSlotDialMs      int64
	FirstFrameMs        int64
	ReceiverWriteMs     int64
	DurabilitySyncMs    int64
	ResumePersistMs     int64
	AckWaitMs           int64
	FinalHashMs         int64
	DestinationCommitMs int64
	MetadataCommitMs    int64
	DataTransferMs      int64
	FinalizationMs      int64
	TotalDurationMs     int64
	ReconnectCount      int
	RetransmittedBytes  int64
	CheckpointCount     int
}

func transferErrorCode(err error) TransferErrorCode {
	code, _ := transferErrorInfo(err)
	return code
}

func (e *Engine) recordTransferStage(attachmentID, peerID, stage string, slotID int, address string, duration time.Duration, bytes int64, retryable bool, code TransferErrorCode) {
	if e == nil || attachmentID == "" {
		return
	}
	ms := duration.Milliseconds()
	e.transferStagesMu.Lock()
	if e.transferStages == nil {
		e.transferStages = make(map[string]transferStageMetrics)
	}
	metrics := e.transferStages[attachmentID]
	switch stage {
	case "control_dial":
		metrics.ControlDialMs += ms
	case "offer_wait":
		metrics.OfferWaitMs += ms
	case "data_slot_dial":
		metrics.DataSlotDialMs += ms
	case "first_frame":
		if metrics.FirstFrameMs == 0 || ms < metrics.FirstFrameMs {
			metrics.FirstFrameMs = ms
		}
	case "receiver_write":
		metrics.ReceiverWriteMs += ms
	case "durability_sync":
		metrics.DurabilitySyncMs += ms
	case "resume_persist":
		metrics.ResumePersistMs += ms
	case "ack_wait":
		metrics.AckWaitMs += ms
	case "final_hash":
		metrics.FinalHashMs += ms
	case "destination_commit":
		metrics.DestinationCommitMs += ms
	case "metadata_commit":
		metrics.MetadataCommitMs += ms
	case "data_transfer":
		metrics.DataTransferMs += ms
	case "finalization":
		metrics.FinalizationMs += ms
	case "total":
		metrics.TotalDurationMs += ms
	case "reconnect":
		metrics.ReconnectCount++
	case "retransmit":
		metrics.RetransmittedBytes += bytes
	case "checkpoint":
		metrics.CheckpointCount++
	}
	e.transferStages[attachmentID] = metrics
	e.transferStagesMu.Unlock()
	log.Printf("传输阶段: attachment=%s peer=%s stage=%s slot=%d address=%s duration=%s bytes=%d retryable=%t code=%s", attachmentID, peerID, stage, slotID, redactDiagnosticText(address), duration, bytes, retryable, code)
}

func (e *Engine) transferStageSnapshot(attachmentID string) transferStageMetrics {
	if e == nil || attachmentID == "" {
		return transferStageMetrics{}
	}
	e.transferStagesMu.Lock()
	defer e.transferStagesMu.Unlock()
	return e.transferStages[attachmentID]
}
