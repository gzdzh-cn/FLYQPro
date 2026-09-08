package chat

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"
)

type transferScheduler struct {
	global chan struct{}
	mu     sync.Mutex
	peers  map[string]chan struct{}
}

func newTransferScheduler() *transferScheduler {
	return &transferScheduler{global: make(chan struct{}, 2), peers: make(map[string]chan struct{})}
}

func (s *transferScheduler) peer(deviceID string) chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.peers[deviceID] == nil {
		s.peers[deviceID] = make(chan struct{}, 1)
	}
	return s.peers[deviceID]
}

func acquireTransferSlot(ctx context.Context, slot chan struct{}, cancel <-chan struct{}) error {
	select {
	case slot <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-cancel:
		return errAttachmentCanceled
	}
}

func (s *transferScheduler) acquirePeer(ctx context.Context, deviceID string, cancel <-chan struct{}) (func(), error) {
	peer := s.peer(deviceID)
	if err := acquireTransferSlot(ctx, peer, cancel); err != nil {
		return nil, err
	}
	return func() { <-peer }, nil
}

func (s *transferScheduler) acquireGlobal(ctx context.Context, cancel <-chan struct{}) (func(), error) {
	if err := acquireTransferSlot(ctx, s.global, cancel); err != nil {
		return nil, err
	}
	return func() { <-s.global }, nil
}

func retryDelay(attempt int) time.Duration {
	if attempt >= 5 {
		return 30 * time.Second
	}
	return time.Duration(1<<maxInt(0, attempt)) * time.Second
}

func transientTransferError(err error) bool {
	if err == nil || errors.Is(err, errAttachmentCanceled) || errors.Is(err, errAttachmentRejected) || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ETIMEDOUT) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return true
	}
	text := strings.ToUpper(err.Error())
	return strings.Contains(text, "CONNECTION") || strings.Contains(text, "连接") || strings.Contains(text, "EOF") || strings.Contains(text, "超时")
}
