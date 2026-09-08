package chat

import (
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type RangeWriterV3 struct {
	mu                  sync.Mutex
	partPath, finalPath string
	size                int64
	done                []ByteRange
	file                *os.File
}

func NewRangeWriterV3(finalPath string, size int64) (*RangeWriterV3, error) {
	if size < 0 {
		return nil, errors.New("invalid file size")
	}
	part := finalPath + ".part"
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(part, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Truncate(size); err != nil {
		f.Close()
		return nil, err
	}
	return &RangeWriterV3{partPath: part, finalPath: finalPath, size: size, file: f}, nil
}
func (w *RangeWriterV3) WriteChunk(offset int64, payload, expectedHash []byte) (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if offset < 0 || offset+int64(len(payload)) > w.size {
		return false, errors.New("chunk outside file")
	}
	sum := sha256.Sum256(payload)
	if len(expectedHash) != len(sum) || !equalBytes(sum[:], expectedHash) {
		return false, errors.New("chunk hash mismatch")
	}
	for _, r := range w.done {
		if offset >= r.Start && offset+int64(len(payload)) <= r.End {
			return true, nil
		}
	}
	if _, err := w.file.WriteAt(payload, offset); err != nil {
		return false, err
	}
	w.done = MergeRanges(append(w.done, ByteRange{offset, offset + int64(len(payload))}))
	return true, nil
}
func (w *RangeWriterV3) Completed() []ByteRange {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]ByteRange(nil), w.done...)
}
func (w *RangeWriterV3) Complete(expected []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.done) != 1 || w.done[0].Start != 0 || w.done[0].End != w.size {
		return errors.New("file ranges incomplete")
	}
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(h, w.file); err != nil {
		return err
	}
	if len(expected) != sha256.Size || !equalBytes(h.Sum(nil), expected) {
		return errors.New("file hash mismatch")
	}
	if err := w.file.Sync(); err != nil {
		return err
	}
	if err := w.file.Close(); err != nil {
		return err
	}
	return os.Rename(w.partPath, w.finalPath)
}
func (w *RangeWriterV3) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
