package chat

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

func (e *Engine) receiveSharedV3File(ctx context.Context, peer Peer, response wireMessage, conn net.Conn, decoder *json.Decoder, target string, cancel <-chan struct{}, progress func(int64)) error {
	if response.TransferMode != v3TransferMode || response.TransferID == "" {
		return fmt.Errorf("v3 shared response required")
	}
	conversationID, err := EnsureConversation(ctx, peer.DeviceID)
	if err != nil {
		return err
	}
	messageID := "shared-" + response.TransferID
	message := Message{MessageID: messageID, ConversationID: conversationID, SenderDeviceID: peer.DeviceID, Kind: "file", Content: response.FileName, Status: "receiving", CreatedAt: nowString(), AttachmentID: response.TransferID, AttachmentName: response.FileName, AttachmentSize: response.FileSize, AttachmentMime: response.MimeType}
	attachment := Attachment{AttachmentID: response.TransferID, MessageID: messageID, FileName: response.FileName, FileSize: response.FileSize, SHA256: response.SHA256, MimeType: response.MimeType}
	if err := e.beginIncomingFile(message, attachment, peer.DeviceID, newWireSession(conn), target, false); err != nil {
		return err
	}
	e.mu.RLock()
	incoming := e.incoming[response.TransferID]
	e.mu.RUnlock()
	if incoming == nil {
		return verifyFile(target, response.FileSize, response.SHA256)
	}
	controlErrors := make(chan error, 1)
	go func() {
		for {
			var msg wireMessage
			err := decoder.Decode(&msg)
			if err != nil {
				controlErrors <- err
				return
			}
			if msg.Type == "share_error" {
				controlErrors <- fmt.Errorf("%s", msg.Status)
				return
			}
		}
	}()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case result := <-incoming.v3Done:
			if result != "completed" {
				return fmt.Errorf("shared v3 verification or commit failed")
			}
			return nil
		case err := <-controlErrors:
			return err
		case <-ctx.Done():
			e.pauseIncomingFile(response.TransferID, "NETWORK_UNSTABLE")
			return ctx.Err()
		case <-cancel:
			e.pauseIncomingFile(response.TransferID, "PAUSED")
			return context.Canceled
		case <-ticker.C:
			if progress != nil {
				incoming.v3Mu.Lock()
				n := incoming.received
				incoming.v3Mu.Unlock()
				progress(n)
			}
		}
	}
}

func (e *Engine) receiveSharedV3Preview(ctx context.Context, peer Peer, response wireMessage, conn net.Conn, decoder *json.Decoder, offset int64, sink func([]byte) error) error {
	if response.TransferMode != v3TransferMode || response.TransferID == "" {
		return fmt.Errorf("v3 shared preview required")
	}
	if sink == nil {
		sink = func([]byte) error { return nil }
	}
	transfer := &incomingFile{attachmentID: response.TransferID, senderID: peer.DeviceID, expected: response.FileSize, received: offset, sha256: response.SHA256, v3StartOffset: offset, v3Sink: sink, digest: sha256.New(), v3Done: make(chan string, 1)}
	if offset > 0 {
		transfer.v3Ranges = []ByteRange{{Start: 0, End: offset}}
	}
	e.mu.Lock()
	e.incoming[response.TransferID] = transfer
	e.mu.Unlock()
	defer func() { e.mu.Lock(); delete(e.incoming, response.TransferID); e.mu.Unlock() }()
	errors := make(chan error, 1)
	go func() {
		for {
			var msg wireMessage
			err := decoder.Decode(&msg)
			if err != nil {
				errors <- err
				return
			}
			if msg.Type == "share_error" {
				errors <- fmt.Errorf("%s", msg.Status)
				return
			}
		}
	}()
	select {
	case result := <-transfer.v3Done:
		if result != "completed" {
			return fmt.Errorf("v3 preview failed")
		}
		return nil
	case err := <-errors:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
