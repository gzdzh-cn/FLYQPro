package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"flyqpro/internal/service/db"
)

const (
	transferResumeVersion = 1
	transferResumeTTL     = 24 * time.Hour
)

type transferResumeState struct {
	Version         int               `json:"version"`
	TransferID      string            `json:"transferId"`
	AttachmentID    string            `json:"attachmentId"`
	MessageID       string            `json:"messageId"`
	SenderDeviceID  string            `json:"senderDeviceId"`
	Direction       string            `json:"direction"`
	SessionID       string            `json:"sessionId"`
	Generation      uint64            `json:"generation"`
	CheckpointSeq   uint64            `json:"checkpointSeq"`
	FileName        string            `json:"fileName"`
	FileSize        int64             `json:"fileSize"`
	SHA256          string            `json:"sha256"`
	SourceMTimeNS   int64             `json:"sourceMtimeNs"`
	Retries         int               `json:"retries"`
	ErrorCode       TransferErrorCode `json:"errorCode"`
	Retryable       bool              `json:"retryable"`
	TempPath        string            `json:"tempPath"`
	TargetPath      string            `json:"targetPath"`
	TransferMode    string            `json:"transferMode"`
	State           TransferState     `json:"state"`
	CompletedRanges []TransferRange   `json:"completedRanges"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

func validTransferIdentifier(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func transferResumePaths(attachmentID string) (string, string, error) {
	if !validTransferIdentifier(attachmentID) {
		return "", "", fmt.Errorf("附件 ID 无效")
	}
	root := filepath.Join(AppDataDir(), "temp")
	return filepath.Join(root, attachmentID+".part"), filepath.Join(root, attachmentID+".resume.json"), nil
}

func normalizeTransferRanges(ranges []TransferRange, size int64) []TransferRange {
	valid := make([]TransferRange, 0, len(ranges))
	for _, item := range ranges {
		if item.Offset < 0 || item.Length <= 0 || item.Offset >= size {
			continue
		}
		if item.Length > size-item.Offset {
			item.Length = size - item.Offset
		}
		valid = append(valid, item)
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].Offset < valid[j].Offset })
	merged := make([]TransferRange, 0, len(valid))
	for _, item := range valid {
		if len(merged) == 0 || item.Offset > merged[len(merged)-1].Offset+merged[len(merged)-1].Length {
			merged = append(merged, item)
			continue
		}
		last := &merged[len(merged)-1]
		if end := item.Offset + item.Length; end > last.Offset+last.Length {
			last.Length = end - last.Offset
		}
	}
	return merged
}

func contiguousTransferOffset(ranges []TransferRange, size int64) int64 {
	normalized := normalizeTransferRanges(ranges, size)
	if len(normalized) == 0 || normalized[0].Offset != 0 {
		return 0
	}
	return normalized[0].Length
}

func saveTransferResumeState(state transferResumeState) error {
	persistence := currentTransferPersistenceIO()
	_, path, err := transferResumePaths(state.AttachmentID)
	if err != nil {
		return err
	}
	if err := persistence.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	state.Version = transferResumeVersion
	if state.State == "" {
		state.State = TransferActive
	}
	if state.Direction == "" {
		state.Direction = "receive"
	}
	// A previous build may have committed only the compatibility sidecar. Use
	// its sequence as well so fallback-only checkpoints remain monotonic.
	if data, readErr := os.ReadFile(path); readErr == nil {
		var current transferResumeState
		if json.Unmarshal(data, &current) == nil && current.AttachmentID == state.AttachmentID && current.CheckpointSeq >= state.CheckpointSeq {
			state.CheckpointSeq = current.CheckpointSeq + 1
		}
	}
	if current, loadErr := loadTransferResumeRecord(context.Background(), state.AttachmentID); loadErr == nil && current.CheckpointSeq >= state.CheckpointSeq {
		state.CheckpointSeq = current.CheckpointSeq + 1
	} else if state.CheckpointSeq == 0 {
		state.CheckpointSeq = 1
	}
	state.CompletedRanges = normalizeTransferRanges(state.CompletedRanges, state.FileSize)
	state.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	// Persist the database record first. The sidecar remains a compatibility
	// fallback for older builds and for a temporarily unavailable database.
	dbErr := persistence.SaveResumeRecord(context.Background(), state)
	temporary := path + ".tmp"
	var sidecarErr error
	file, err := persistence.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		sidecarErr = err
	} else {
		if _, err := file.Write(data); err != nil {
			sidecarErr = err
		} else if err := persistence.SyncFile(file); err != nil {
			sidecarErr = err
		}
		if closeErr := file.Close(); sidecarErr == nil && closeErr != nil {
			sidecarErr = closeErr
		}
		if sidecarErr == nil {
			sidecarErr = persistence.Rename(temporary, path)
			if sidecarErr == nil {
				// A rename is only durable after the containing directory is
				// synchronized. This closes the crash window where the JSON
				// contents exist but the directory entry does not.
				sidecarErr = persistence.SyncDirectory(filepath.Dir(path))
			}
		}
		if sidecarErr != nil {
			_ = file.Close()
			_ = persistence.Remove(temporary)
		}
	}
	if dbErr != nil && sidecarErr != nil {
		_ = db.SetTransferResumeMigrationStatus(context.Background(), "rollback", state.AttachmentID, dbErr.Error()+"; "+sidecarErr.Error())
		return fmt.Errorf("恢复记录持久化失败: sqlite=%v; sidecar=%v", dbErr, sidecarErr)
	}
	if dbErr == nil {
		_ = db.SetTransferResumeMigrationStatus(context.Background(), "migrating", state.AttachmentID, "")
	} else {
		_ = db.SetTransferResumeMigrationStatus(context.Background(), "rollback", state.AttachmentID, dbErr.Error())
	}
	return nil
}

func syncResumeDirectory(path string) error {
	return currentTransferPersistenceIO().SyncDirectory(path)
}

func loadTransferResumeState(attachmentID string) (transferResumeState, error) {
	partPath, path, err := transferResumePaths(attachmentID)
	if err != nil {
		return transferResumeState{}, err
	}
	info, statErr := os.Stat(partPath)
	partSize := int64(-1)
	if statErr == nil && !info.IsDir() {
		partSize = info.Size()
	}
	data, readErr := os.ReadFile(path)
	var sidecarState transferResumeState
	if readErr == nil {
		if unmarshalErr := json.Unmarshal(data, &sidecarState); unmarshalErr != nil {
			readErr = unmarshalErr
		} else if validationErr := validateTransferResumeCandidateWithSource(sidecarState, attachmentID, partSize); validationErr != nil {
			readErr = validationErr
		}
	}
	databaseState, databaseErr := loadTransferResumeRecord(context.Background(), attachmentID)
	if databaseErr == nil {
		databaseErr = validateTransferResumeCandidateWithSource(databaseState, attachmentID, partSize)
	}
	if readErr != nil && databaseErr != nil {
		return transferResumeState{}, fmt.Errorf("恢复状态无效: sidecar=%v; sqlite=%v", readErr, databaseErr)
	}
	state := sidecarState
	if databaseErr == nil && (readErr != nil || databaseState.CheckpointSeq > sidecarState.CheckpointSeq) {
		state = databaseState
	}
	state.CompletedRanges = normalizeTransferRanges(state.CompletedRanges, state.FileSize)
	if databaseErr == nil {
		_ = db.SetTransferResumeMigrationStatus(context.Background(), "verified", attachmentID, "")
	} else if readErr != nil {
		_ = db.SetTransferResumeMigrationStatus(context.Background(), "rollback", attachmentID, databaseErr.Error())
	}
	return state, nil
}

func validateTransferResumeCandidate(state transferResumeState, attachmentID string, partSize int64) error {
	if state.Version != transferResumeVersion || state.AttachmentID != attachmentID || state.FileSize < 0 || time.Since(state.UpdatedAt) > transferResumeTTL {
		return fmt.Errorf("版本、身份或有效期无效")
	}
	for _, item := range normalizeTransferRanges(state.CompletedRanges, state.FileSize) {
		if item.Offset+item.Length > partSize {
			return fmt.Errorf("恢复范围超过临时文件长度")
		}
	}
	return nil
}

func validateTransferResumeCandidateWithSource(state transferResumeState, attachmentID string, partSize int64) error {
	if state.Direction == "send" {
		if state.TargetPath == "" {
			return fmt.Errorf("发送恢复源文件路径缺失")
		}
		info, err := os.Stat(state.TargetPath)
		if err != nil || info.IsDir() {
			return fmt.Errorf("发送恢复源文件无效")
		}
		if outgoingResumeSourceChanged(state, info) {
			return fmt.Errorf("发送恢复源文件已变化")
		}
		if partSize < 0 {
			partSize = info.Size()
		}
	}
	if partSize < 0 {
		return fmt.Errorf("恢复临时文件无效")
	}
	return validateTransferResumeCandidate(state, attachmentID, partSize)
}

func removeTransferResumeArtifacts(attachmentID string, removePart, removeSidecar bool) {
	part, state, err := transferResumePaths(attachmentID)
	if err != nil {
		return
	}
	if removeSidecar {
		_ = os.Remove(state)
		_ = os.Remove(state + ".tmp")
	}
	if removePart {
		_ = os.Remove(part)
	}
}

func removeTransferResumeState(attachmentID string, removePart bool) {
	removeTransferResumeArtifacts(attachmentID, removePart, true)
}

func cleanupExpiredTransferResumeStates() {
	root := filepath.Join(AppDataDir(), "temp")
	paths, err := filepath.Glob(filepath.Join(root, "*.resume.json"))
	if err != nil {
		return
	}
	for _, path := range paths {
		attachmentID := strings.TrimSuffix(filepath.Base(path), ".resume.json")
		if !validTransferIdentifier(attachmentID) {
			continue
		}
		if _, err := loadTransferResumeState(attachmentID); err != nil {
			_ = markTransferResumeTerminal(context.Background(), attachmentID, TransferFailed, ErrSessionNotReady, false)
		}
	}
	if records, err := listTransferResumeRecords(context.Background()); err == nil {
		for _, record := range records {
			if record.State == TransferCompleted || record.State == TransferCancelled || record.State == TransferFailed || record.Direction == "send" {
				continue
			}
			if _, err := loadTransferResumeState(record.AttachmentID); err != nil {
				_ = markTransferResumeTerminal(context.Background(), record.AttachmentID, TransferFailed, ErrSessionNotReady, false)
			}
		}
	}
}

func transferResumeMatches(state transferResumeState, messageID, senderID string, size int64, sum string) bool {
	return state.MessageID == messageID && state.SenderDeviceID == senderID && state.FileSize == size && strings.EqualFold(state.SHA256, sum)
}

func outgoingResumeSourceChanged(state transferResumeState, info os.FileInfo) bool {
	if info == nil || state.Direction != "send" || state.SourceMTimeNS == 0 {
		return false
	}
	return state.FileSize != info.Size() || state.SourceMTimeNS != info.ModTime().UnixNano()
}

func persistOutgoingResumeState(state transferResumeState) error {
	if state.Direction == "" {
		state.Direction = "send"
	}
	if state.State == "" {
		state.State = TransferQueued
	}
	if state.CheckpointSeq == 0 {
		state.CheckpointSeq = 1
	} else {
		state.CheckpointSeq++
	}
	// Outgoing checkpoints use the same SQLite + atomic sidecar contract as
	// incoming transfers. Keeping both records lets a sender recover after a
	// database outage without silently losing the last remote confirmation.
	return saveTransferResumeState(state)
}

func (e *Engine) persistOutgoingConfirmedRange(attachmentID string, start, end int64, sessionID string, generation uint64) error {
	if end <= start {
		return nil
	}
	e.mu.RLock()
	transfer := e.outgoing[attachmentID]
	e.mu.RUnlock()
	if transfer == nil {
		// Protocol unit tests and preview transports can run without a managed
		// outgoing task. Production sends always register one first.
		return nil
	}
	transfer.resumeMu.Lock()
	defer transfer.resumeMu.Unlock()
	state := transfer.resumeState
	if state.AttachmentID == "" {
		return nil
	}
	state.SessionID = sessionID
	state.Generation = generation
	state.State = TransferActive
	state.CompletedRanges = append(state.CompletedRanges, TransferRange{Offset: start, Length: end - start})
	state.CompletedRanges = normalizeTransferRanges(state.CompletedRanges, state.FileSize)
	var covered int64
	for _, item := range state.CompletedRanges {
		covered += item.Length
	}
	transfer.resumeState = state
	// Keep SQLite checkpoints bounded: acknowledged chunks are idempotent and
	// can be resent safely, so persisting every 4 MiB (and always the final
	// range) preserves crash recovery without turning each chunk into a DB fsync.
	if covered < state.FileSize && covered-transfer.resumePersistedBytes < 4*1024*1024 {
		return nil
	}
	state.CheckpointSeq++
	if err := saveTransferResumeState(state); err != nil {
		return err
	}
	state.UpdatedAt = time.Now().UTC()
	transfer.resumeState = state
	transfer.resumePersistedBytes = covered
	return nil
}
