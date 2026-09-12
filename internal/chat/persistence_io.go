package chat

import (
	"context"
	"io/fs"
	"os"
	"runtime"
	"sync"
)

// transferPersistenceIO is the single durability boundary for the v3 data
// plane. Production uses the operating system implementation below; tests can
// replace it to deterministically exercise fsync, metadata and commit faults.
// It deliberately remains package-private and is not part of the Wails API.
type transferPersistenceIO interface {
	MkdirAll(string, fs.FileMode) error
	Open(string) (*os.File, error)
	OpenFile(string, int, fs.FileMode) (*os.File, error)
	CreateTemp(string, string) (*os.File, error)
	Remove(string) error
	Rename(string, string) error
	SyncFile(*os.File) error
	SyncDirectory(string) error
	SaveResumeRecord(context.Context, transferResumeState) error
}

type osTransferPersistenceIO struct{}

func (osTransferPersistenceIO) MkdirAll(path string, mode fs.FileMode) error {
	return os.MkdirAll(path, mode)
}
func (osTransferPersistenceIO) Open(path string) (*os.File, error) { return os.Open(path) }
func (osTransferPersistenceIO) OpenFile(path string, flag int, mode fs.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag, mode)
}
func (osTransferPersistenceIO) CreateTemp(directory, pattern string) (*os.File, error) {
	return os.CreateTemp(directory, pattern)
}
func (osTransferPersistenceIO) Remove(path string) error           { return os.Remove(path) }
func (osTransferPersistenceIO) Rename(source, target string) error { return os.Rename(source, target) }
func (osTransferPersistenceIO) SyncFile(file *os.File) error       { return file.Sync() }
func (osTransferPersistenceIO) SaveResumeRecord(ctx context.Context, state transferResumeState) error {
	return saveTransferResumeRecord(ctx, state)
}
func (osTransferPersistenceIO) SyncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr := directory.Close()
	if err != nil {
		return err
	}
	return closeErr
}

var transferPersistenceState = struct {
	sync.RWMutex
	value transferPersistenceIO
}{value: osTransferPersistenceIO{}}

func currentTransferPersistenceIO() transferPersistenceIO {
	transferPersistenceState.RLock()
	value := transferPersistenceState.value
	transferPersistenceState.RUnlock()
	return value
}

// replaceTransferPersistenceIO is intentionally unexported. Tests must restore
// the returned previous implementation before completing.
func replaceTransferPersistenceIO(value transferPersistenceIO) func() {
	if value == nil {
		value = osTransferPersistenceIO{}
	}
	transferPersistenceState.Lock()
	previous := transferPersistenceState.value
	transferPersistenceState.value = value
	transferPersistenceState.Unlock()
	return func() {
		transferPersistenceState.Lock()
		transferPersistenceState.value = previous
		transferPersistenceState.Unlock()
	}
}

func syncTransferFile(file *os.File) error {
	return currentTransferPersistenceIO().SyncFile(file)
}
