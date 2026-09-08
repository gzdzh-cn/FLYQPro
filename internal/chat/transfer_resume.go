package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	transferResumeVersion = 1
	transferResumeTTL     = 24 * time.Hour
)

type transferResumeState struct {
	Version         int             `json:"version"`
	TransferID      string          `json:"transferId"`
	AttachmentID    string          `json:"attachmentId"`
	MessageID       string          `json:"messageId"`
	SenderDeviceID  string          `json:"senderDeviceId"`
	FileName        string          `json:"fileName"`
	FileSize        int64           `json:"fileSize"`
	SHA256          string          `json:"sha256"`
	TempPath        string          `json:"tempPath"`
	TargetPath      string          `json:"targetPath"`
	TransferMode    string          `json:"transferMode"`
	CompletedRanges []TransferRange `json:"completedRanges"`
	UpdatedAt       time.Time       `json:"updatedAt"`
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
	_, path, err := transferResumePaths(state.AttachmentID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	state.Version = transferResumeVersion
	state.CompletedRanges = normalizeTransferRanges(state.CompletedRanges, state.FileSize)
	state.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(temporary)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func loadTransferResumeState(attachmentID string) (transferResumeState, error) {
	partPath, path, err := transferResumePaths(attachmentID)
	if err != nil {
		return transferResumeState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return transferResumeState{}, err
	}
	var state transferResumeState
	if err := json.Unmarshal(data, &state); err != nil {
		return transferResumeState{}, err
	}
	if state.Version != transferResumeVersion || state.AttachmentID != attachmentID || state.FileSize < 0 || time.Since(state.UpdatedAt) > transferResumeTTL {
		return transferResumeState{}, fmt.Errorf("恢复状态无效或已过期")
	}
	state.CompletedRanges = normalizeTransferRanges(state.CompletedRanges, state.FileSize)
	info, statErr := os.Stat(partPath)
	if statErr != nil || info.IsDir() || info.Size() < contiguousTransferOffset(state.CompletedRanges, state.FileSize) {
		return transferResumeState{}, fmt.Errorf("恢复临时文件无效")
	}
	return state, nil
}

func removeTransferResumeState(attachmentID string, removePart bool) {
	part, state, err := transferResumePaths(attachmentID)
	if err != nil {
		return
	}
	_ = os.Remove(state)
	_ = os.Remove(state + ".tmp")
	if removePart {
		_ = os.Remove(part)
	}
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
			removeTransferResumeState(attachmentID, true)
		}
	}
}

func transferResumeMatches(state transferResumeState, messageID, senderID string, size int64, sum string) bool {
	return state.MessageID == messageID && state.SenderDeviceID == senderID && state.FileSize == size && strings.EqualFold(state.SHA256, sum)
}
