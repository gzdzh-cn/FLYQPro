package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// TransferSnapshot is the stable, transport-neutral diagnostic view exposed to
// the desktop UI. It deliberately omits sockets and filesystem internals.
type TransferSnapshot struct {
	AttachmentID        string            `json:"attachmentId"`
	TransferID          string            `json:"transferId"`
	MessageID           string            `json:"messageId"`
	PeerDeviceID        string            `json:"peerDeviceId"`
	Direction           string            `json:"direction"`
	SessionID           string            `json:"sessionId"`
	Generation          uint64            `json:"generation"`
	MetricGeneration    uint64            `json:"metricGeneration,omitempty"`
	State               TransferState     `json:"state"`
	Phase               string            `json:"phase,omitempty"`
	Transferred         int64             `json:"transferred"`
	DurableBytes        int64             `json:"durableBytes"`
	Total               int64             `json:"total"`
	Percent             int               `json:"percent"`
	Speed               float64           `json:"speed,omitempty"`
	AverageSpeed        float64           `json:"averageSpeed,omitempty"`
	PeakSpeed           float64           `json:"peakSpeed,omitempty"`
	LocalSendSpeed      float64           `json:"localSendSpeed,omitempty"`
	ETAs                int64             `json:"etaSeconds,omitempty"`
	ElapsedMs           int64             `json:"elapsedMs,omitempty"`
	MetricStartedBytes  int64             `json:"metricStartedBytes,omitempty"`
	MetricLastBytes     int64             `json:"metricLastBytes,omitempty"`
	MetricSeq           uint64            `json:"metricSeq,omitempty"`
	CheckpointSeq       uint64            `json:"checkpointSeq,omitempty"`
	Retries             int               `json:"retries"`
	ErrorCode           TransferErrorCode `json:"errorCode,omitempty"`
	Retryable           bool              `json:"retryable"`
	Verified            *bool             `json:"verified,omitempty"`
	DiskWriteMs         int64             `json:"diskWriteMs,omitempty"`
	AckLatencyMs        int64             `json:"ackLatencyMs,omitempty"`
	ChunkSize           int               `json:"chunkSize,omitempty"`
	WindowSize          int               `json:"windowSize,omitempty"`
	WindowBytes         int64             `json:"windowBytes,omitempty"`
	InFlightBytes       int64             `json:"inFlightBytes,omitempty"`
	AckTargetBytes      int64             `json:"ackTargetBytes,omitempty"`
	StreamCount         int               `json:"streamCount,omitempty"`
	ActiveStreams       int               `json:"activeStreams,omitempty"`
	TransferMode        string            `json:"transferMode,omitempty"`
	Transport           string            `json:"transport,omitempty"`
	TuningState         string            `json:"tuningState,omitempty"`
	CommittedPath       string            `json:"committedPath,omitempty"`
	ControlDialMs       int64             `json:"controlDialMs,omitempty"`
	OfferWaitMs         int64             `json:"offerWaitMs,omitempty"`
	DataSlotDialMs      int64             `json:"dataSlotDialMs,omitempty"`
	FirstFrameMs        int64             `json:"firstFrameMs,omitempty"`
	ReceiverWriteMs     int64             `json:"receiverWriteMs,omitempty"`
	DurabilitySyncMs    int64             `json:"durabilitySyncMs,omitempty"`
	ResumePersistMs     int64             `json:"resumePersistMs,omitempty"`
	AckWaitMs           int64             `json:"ackWaitMs,omitempty"`
	FinalHashMs         int64             `json:"finalHashMs,omitempty"`
	DestinationCommitMs int64             `json:"destinationCommitMs,omitempty"`
	MetadataCommitMs    int64             `json:"metadataCommitMs,omitempty"`
	DataTransferMs      int64             `json:"dataTransferMs,omitempty"`
	FinalizationMs      int64             `json:"finalizationMs,omitempty"`
	TotalDurationMs     int64             `json:"totalDurationMs,omitempty"`
	ReconnectCount      int               `json:"reconnectCount,omitempty"`
	RetransmittedBytes  int64             `json:"retransmittedBytes,omitempty"`
	UpdatedAt           time.Time         `json:"updatedAt"`
}

func snapshotFromResume(state transferResumeState) TransferSnapshot {
	var durable int64
	for _, r := range normalizeTransferRanges(state.CompletedRanges, state.FileSize) {
		durable += r.Length
	}
	transferID := state.TransferID
	if transferID == "" {
		transferID = state.AttachmentID
	}
	direction := state.Direction
	if direction == "" {
		direction = "receive"
	}
	phase := string(state.State)
	if state.State == TransferActive {
		if direction == "receive" {
			phase = "receiving"
		} else {
			phase = "transferring"
		}
	}
	return TransferSnapshot{AttachmentID: state.AttachmentID, TransferID: transferID, MessageID: state.MessageID, PeerDeviceID: state.SenderDeviceID, Direction: direction, SessionID: state.SessionID, Generation: state.Generation, MetricGeneration: metricGenerationOrDefault(state.MetricGeneration), State: state.State, Phase: phase, Transferred: durable, DurableBytes: durable, Total: state.FileSize, Percent: transferProgressPercent(durable, state.FileSize, phase), ElapsedMs: state.ElapsedMs, MetricSeq: state.MetricSeq, CheckpointSeq: state.CheckpointSeq, MetricStartedBytes: state.MetricStartedBytes, MetricLastBytes: state.MetricLastBytes, Retries: state.Retries, ErrorCode: state.ErrorCode, Retryable: state.Retryable, UpdatedAt: state.UpdatedAt}
}

func snapshotFromProgress(value map[string]any) (TransferSnapshot, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return TransferSnapshot{}, err
	}
	var snapshot TransferSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return TransferSnapshot{}, err
	}
	if snapshot.AttachmentID == "" {
		return TransferSnapshot{}, fmt.Errorf("transfer snapshot missing attachmentId")
	}
	return snapshot, nil
}

// ListActiveTransfers returns in-memory work plus durable recovery records.
func (e *Engine) ListActiveTransfers() []TransferSnapshot {
	seen := make(map[string]TransferSnapshot)
	if rows, err := listTransferResumeRecords(context.Background()); err == nil {
		for _, row := range rows {
			if row.State == TransferCompleted || row.State == TransferCancelled || row.State == TransferFailed {
				continue
			}
			seen[row.AttachmentID] = snapshotFromResume(row)
		}
	}
	e.mu.RLock()
	for id, transfer := range e.incoming {
		if transfer == nil {
			continue
		}
		s := snapshotFromResume(transfer.resumeState)
		s.AttachmentID = id
		s.TransferID, s.MessageID, s.PeerDeviceID, s.Direction, s.Total = id, transfer.messageID, transfer.senderID, "receive", transfer.expected
		s.Transferred, s.DurableBytes = transfer.received, transfer.durableBytes
		s.State, s.UpdatedAt = TransferActive, time.Now().UTC()
		if persisted, err := loadTransferSnapshotDirection(context.Background(), id, "receive"); err == nil {
			s = mergeTransferSnapshot(persisted, s)
		}
		seen[id] = s
	}
	for id, transfer := range e.outgoing {
		if transfer == nil {
			continue
		}
		s := TransferSnapshot{AttachmentID: id, TransferID: id, MessageID: transfer.message.MessageID, PeerDeviceID: transfer.peerID, Direction: "send", State: TransferActive, Phase: "transferring", Total: transfer.message.AttachmentSize, Transferred: e.lastTransferBytes(id, "send"), UpdatedAt: time.Now().UTC()}
		if persisted, err := loadTransferSnapshotDirection(context.Background(), id, "send"); err == nil {
			s = mergeTransferSnapshot(persisted, s)
		}
		seen[id] = s
	}
	e.mu.RUnlock()
	result := make([]TransferSnapshot, 0, len(seen))
	for _, item := range seen {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.Before(result[j].UpdatedAt) })
	return result
}

func (e *Engine) ListRecoveryTasks() []TransferSnapshot {
	rows, err := listTransferResumeRecords(context.Background())
	if err != nil {
		return nil
	}
	result := make([]TransferSnapshot, 0, len(rows))
	for _, row := range rows {
		if row.State == TransferPausedLocal || row.State == TransferPausedPeer || row.State == TransferPausedNetwork || row.State == TransferQueued || (row.State == TransferFailed && row.Retryable) {
			result = append(result, snapshotFromResume(row))
		}
	}
	return result
}

func (e *Engine) GetTransferDiagnostics(transferID string) (TransferSnapshot, error) {
	send, sendErr := loadTransferSnapshotDirection(context.Background(), transferID, "send")
	remote, remoteErr := loadTransferSnapshotDirection(context.Background(), transferID, "remote-receive")
	if sendErr == nil {
		if remoteErr == nil {
			return mergeSenderDiagnostics(send, remote), nil
		}
		return send, nil
	}
	if snapshot, snapshotErr := loadTransferSnapshotDirection(context.Background(), transferID, "receive"); snapshotErr == nil {
		return snapshot, nil
	}
	if remoteErr == nil {
		return remote, nil
	}
	if snapshot, snapshotErr := loadTransferSnapshot(context.Background(), transferID); snapshotErr == nil {
		return snapshot, nil
	}
	for _, item := range e.ListActiveTransfers() {
		if item.TransferID == transferID || item.AttachmentID == transferID {
			return item, nil
		}
	}
	rows, err := listTransferResumeRecords(context.Background())
	if err == nil {
		for _, row := range rows {
			if row.TransferID == transferID || row.AttachmentID == transferID {
				return snapshotFromResume(row), nil
			}
		}
	}
	return TransferSnapshot{}, fmt.Errorf("transfer not found")
}

func mergeSenderDiagnostics(send, remote TransferSnapshot) TransferSnapshot {
	result := remote
	result.Direction = "send"
	result.State, result.Phase = send.State, send.Phase
	result.ErrorCode, result.Retryable, result.Retries = send.ErrorCode, send.Retryable, send.Retries
	result.LocalSendSpeed = send.LocalSendSpeed
	if result.MessageID == "" {
		result.MessageID = send.MessageID
	}
	if result.PeerDeviceID == "" {
		result.PeerDeviceID = send.PeerDeviceID
	}
	if result.Total == 0 {
		result.Total = send.Total
	}
	if result.ControlDialMs == 0 {
		result.ControlDialMs = send.ControlDialMs
	}
	if result.DataSlotDialMs == 0 {
		result.DataSlotDialMs = send.DataSlotDialMs
	}
	if result.FirstFrameMs == 0 {
		result.FirstFrameMs = send.FirstFrameMs
	}
	if result.ReconnectCount == 0 {
		result.ReconnectCount = send.ReconnectCount
	}
	if result.RetransmittedBytes == 0 {
		result.RetransmittedBytes = send.RetransmittedBytes
	}
	return result
}

func mergeTransferSnapshot(primary, fallback TransferSnapshot) TransferSnapshot {
	if primary.AttachmentID == "" {
		return fallback
	}
	if primary.ElapsedMs < fallback.ElapsedMs {
		primary.ElapsedMs = fallback.ElapsedMs
	}
	if primary.MetricGeneration == 0 {
		primary.MetricGeneration = fallback.MetricGeneration
	}
	if primary.Total == 0 {
		primary.Total = fallback.Total
	}
	if primary.Transferred < fallback.Transferred && primary.CheckpointSeq <= fallback.CheckpointSeq {
		primary.Transferred, primary.DurableBytes = fallback.Transferred, fallback.DurableBytes
	}
	if primary.MessageID == "" {
		primary.MessageID = fallback.MessageID
	}
	if primary.Direction == "" {
		primary.Direction = fallback.Direction
	}
	return primary
}

// ListTransferSnapshots restores the latest transfer projection after a
// frontend reload or process restart. It includes completed and failed history.
func (e *Engine) ListTransferSnapshots() []TransferSnapshot {
	items, err := listTransferSnapshots(context.Background())
	if err != nil {
		return nil
	}
	return items
}

func (e *Engine) PauseTransfer(transferID string) error { return e.PauseAttachment(transferID) }

func (e *Engine) ResumeTransfer(ctx context.Context, transferID string) (Message, error) {
	return e.ResumeAttachment(ctx, transferID)
}

func (e *Engine) RetryTransfer(ctx context.Context, transferID string) (Message, error) {
	attachment, err := GetAttachment(ctx, transferID)
	if err != nil {
		return Message{}, err
	}
	return e.RetryAttachment(ctx, attachment.MessageID)
}

func (e *Engine) CancelTransfer(transferID string) error { return e.CancelAttachment(transferID) }
