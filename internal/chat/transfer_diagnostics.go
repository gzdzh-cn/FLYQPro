package chat

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// TransferSnapshot is the stable, transport-neutral diagnostic view exposed to
// the desktop UI. It deliberately omits sockets and filesystem internals.
type TransferSnapshot struct {
	TransferID   string            `json:"transferId"`
	MessageID    string            `json:"messageId"`
	PeerDeviceID string            `json:"peerDeviceId"`
	Direction    string            `json:"direction"`
	SessionID    string            `json:"sessionId"`
	Generation   uint64            `json:"generation"`
	State        TransferState     `json:"state"`
	Transferred  int64             `json:"transferred"`
	DurableBytes int64             `json:"durableBytes"`
	Total        int64             `json:"total"`
	Retries      int               `json:"retries"`
	ErrorCode    TransferErrorCode `json:"errorCode,omitempty"`
	Retryable    bool              `json:"retryable"`
	UpdatedAt    time.Time         `json:"updatedAt"`
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
	return TransferSnapshot{TransferID: transferID, MessageID: state.MessageID, PeerDeviceID: state.SenderDeviceID, Direction: direction, SessionID: state.SessionID, Generation: state.Generation, State: state.State, Transferred: durable, DurableBytes: durable, Total: state.FileSize, Retries: state.Retries, ErrorCode: state.ErrorCode, Retryable: state.Retryable, UpdatedAt: state.UpdatedAt}
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
		s.TransferID, s.MessageID, s.PeerDeviceID, s.Direction, s.Total = id, transfer.messageID, transfer.senderID, "receive", transfer.expected
		s.Transferred, s.DurableBytes = transfer.received, transfer.durableBytes
		s.State, s.UpdatedAt = TransferActive, time.Now().UTC()
		seen[id] = s
	}
	for id, transfer := range e.outgoing {
		if transfer == nil {
			continue
		}
		s := TransferSnapshot{TransferID: id, MessageID: transfer.message.MessageID, PeerDeviceID: transfer.peerID, Direction: "send", State: TransferActive, Total: transfer.message.AttachmentSize, Transferred: e.lastTransferBytes(id, "send"), UpdatedAt: time.Now().UTC()}
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
	for _, item := range e.ListActiveTransfers() {
		if item.TransferID == transferID {
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
