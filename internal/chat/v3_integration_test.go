package chat

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

// Exercise the production sender, receiver, resume store and final rename over
// mutual TLS 1.3. A reusable connection is counted at the actual accept boundary.
func runV3TLS(t *testing.T, sizes []int, streams int, resumed bool) {
	t.Helper()
	dbctx := openSharedFolderTestDatabase(t)
	senderIdentity, senderCertificate := parallelTestIdentity(t)
	receiverIdentity, receiverCertificate := parallelTestIdentity(t)
	senderID := senderIdentity.DeviceID
	if err := UpsertPeer(dbctx, Peer{DeviceID: senderID, Nickname: "sender", Relation: "friend"}); err != nil {
		t.Fatal(err)
	}
	conversationID, err := EnsureConversation(dbctx, senderID)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{receiverCertificate}, MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAnyClientCert})
	if err != nil {
		t.Fatal(err)
	}
	receiver, sender := NewEngine(), NewEngine()
	sender.identity = senderIdentity
	senderPeer := pinnedTestPeer(senderIdentity, senderCertificate)
	senderPeer.Relation = PeerRelation
	receiver.peers[senderID] = senderPeer
	peer := pinnedTestPeer(receiverIdentity, receiverCertificate)
	peer.IP = "127.0.0.1"
	peer.DataPort = listener.Addr().(*net.TCPAddr).Port
	totalBytes := 0
	for _, size := range sizes {
		totalBytes += size
	}
	testTimeout := 90 * time.Second
	if estimated := time.Duration(totalBytes/(8*1024*1024))*time.Second + 2*time.Minute; estimated > testTimeout {
		testTimeout = estimated
	}
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	var accepts atomic.Int32
	var wg sync.WaitGroup
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepts.Add(1)
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				stop := context.AfterFunc(ctx, func() { conn.Close() })
				defer stop()
				_ = receiver.receiveV3DataConnection(conn)
			}()
		}
	}()
	defer func() {
		cancel()
		listener.Close()
		<-acceptDone
		wg.Wait()
		for _, p := range sender.peerPools {
			p.CloseIdle(time.Now().Add(time.Hour), 0)
		}
	}()
	root := t.TempDir()
	type validationSample struct {
		Result          string        `json:"result"`
		Scenario        string        `json:"scenario"`
		Bytes           int           `json:"bytes"`
		DataTimeMS      int64         `json:"dataTimeMs"`
		MiBPerSecond    float64       `json:"mibPerSecond"`
		Slots           int           `json:"slots"`
		SHA256Verified  bool          `json:"sha256Verified"`
		Resumed         bool          `json:"resumed"`
		CanonicalState  TransferState `json:"canonicalState"`
		SessionID       string        `json:"sessionId"`
		Generation      uint64        `json:"generation"`
		CheckpointSeq   uint64        `json:"checkpointSeq"`
		DurableBytes    int64         `json:"durableBytes"`
		FinalFileBytes  int64         `json:"finalFileBytes"`
		PartFilePresent bool          `json:"partFilePresent"`
	}
	samples := make([]validationSample, 0, len(sizes))
	for index, size := range sizes {
		id := fmt.Sprintf("file-%d", index)
		sourcePath, finalPath := filepath.Join(root, id+".src"), filepath.Join(root, id+".out")
		source, err := os.OpenFile(sourcePath, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			t.Fatal(err)
		}
		block := bytes.Repeat([]byte{byte(index + 1)}, 1<<20)
		digest := sha256.New()
		for left := size; left > 0; {
			n := min(left, len(block))
			if _, err := source.Write(block[:n]); err != nil {
				t.Fatal(err)
			}
			_, _ = digest.Write(block[:n])
			left -= n
		}
		var sum [32]byte
		copy(sum[:], digest.Sum(nil))
		message := Message{MessageID: "msg-" + id, AttachmentID: id, AttachmentName: id, AttachmentSize: int64(size), SenderDeviceID: senderID, ConversationID: conversationID, Kind: "file", Status: "receiving", CreatedAt: nowString()}
		attachment := Attachment{AttachmentID: id, MessageID: message.MessageID, FileName: id, FileSize: int64(size), SHA256: hex.EncodeToString(sum[:])}
		if err := receiver.beginIncomingFile(message, attachment, senderID, nil, finalPath, false); err != nil {
			t.Fatal(err)
		}
		var completed []ByteRange
		if resumed && size > 0 {
			r := ByteRange{Start: int64(size / 3), End: int64(size * 2 / 3)}
			if index%2 == 1 {
				r = ByteRange{Start: 0, End: int64(size)}
			}
			transfer := receiver.incoming[id]
			if _, err := transfer.file.Seek(r.Start, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(transfer.file, io.NewSectionReader(source, r.Start, r.End-r.Start)); err != nil {
				t.Fatal(err)
			}
			completed = []ByteRange{r}
			transfer.v3Ranges = completed
			transfer.received = r.End - r.Start
		}
		started := time.Now()
		err = sender.sendV3FileDataParallel(ctx, peer, message, source, attachment.SHA256, streams, 0, completed)
		source.Close()
		if err != nil {
			t.Fatalf("file %d: %v", index, err)
		}
		// Returning success requires receiver verification and rename, not merely
		// finishing the last socket write.
		elapsed := time.Since(started)
		if err := verifyFile(finalPath, int64(size), hex.EncodeToString(sum[:])); err != nil {
			t.Fatalf("file %d not verified/renamed: %v", index, err)
		}
		resume, err := loadTransferResumeRecord(context.Background(), id)
		if err != nil || resume.State != TransferCompleted || resume.SessionID == "" || resume.Generation == 0 {
			t.Fatalf("file %d terminal session checkpoint missing: state=%+v err=%v", index, resume, err)
		}
		finalInfo, err := os.Stat(finalPath)
		if err != nil {
			t.Fatal(err)
		}
		partPath, _, err := transferResumePaths(id)
		if err != nil {
			t.Fatal(err)
		}
		_, partErr := os.Stat(partPath)
		partPresent := partErr == nil
		if partErr != nil && !os.IsNotExist(partErr) {
			t.Fatal(partErr)
		}
		t.Logf("file=%d bytes=%d dataTime=%s MiBps=%.2f slots=%d sha256Verified=true", index, size, elapsed, float64(size)/(1<<20)/elapsed.Seconds(), streams)
		samples = append(samples, validationSample{
			Result: "pass", Scenario: "desktop-desktop-tls-v3", Bytes: size, DataTimeMS: elapsed.Milliseconds(),
			MiBPerSecond: float64(size) / (1 << 20) / elapsed.Seconds(), Slots: streams,
			SHA256Verified: true, Resumed: resumed, CanonicalState: resume.State,
			SessionID: resume.SessionID, Generation: resume.Generation, CheckpointSeq: resume.CheckpointSeq,
			DurableBytes: int64(size), FinalFileBytes: finalInfo.Size(), PartFilePresent: partPresent,
		})

	}
	if got := accepts.Load(); got > int32(streams) {
		t.Fatalf("connections not reused: %d for %d files/%d slots", got, len(sizes), streams)
	}
	t.Logf("files=%d TLS13 connections=%d slots=%d SHA256=verified", len(sizes), accepts.Load(), streams)
	if reportPath := os.Getenv("FLYQPRO_TRANSFER_REPORT"); reportPath != "" {
		encoded, err := json.MarshalIndent(map[string]any{
			"schemaVersion": 1,
			"status":        "pass",
			"transport":     "TLS13/TCP-v3",
			"generatedAt":   time.Now().UTC().Format(time.RFC3339Nano),
			"goVersion":     runtime.Version(),
			"goos":          runtime.GOOS,
			"goarch":        runtime.GOARCH,
			"network":       "IPv4 loopback",
			"buildId":       firstNonEmpty(os.Getenv("FLYQPRO_BUILD_ID"), "local"),
			"commit":        firstNonEmpty(os.Getenv("GIT_COMMIT"), "unknown"),
			"worktree":      firstNonEmpty(os.Getenv("FLYQPRO_WORKTREE_STATUS"), "dirty-or-unknown"),
			"samples":       samples,
		}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(reportPath, append(encoded, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestV3PersistentTLSTenFiles(t *testing.T) {
	runV3TLS(t, []int{0, 1, 17, 1048577, 13, 3000, 0, 9, 2097153, 16}, 1, false)
}
func TestV3SparseAndFullyResumedTLS(t *testing.T) { runV3TLS(t, []int{3*1024*1024 + 7, 100}, 4, true) }

func TestV3RepliesRejectStaleIdentity(t *testing.T) {
	frame := NewChunkFrame([16]byte{1}, 2, 3, 4, []byte("x"))
	frame.SessionID = [16]byte{5}
	frame.Generation = 6
	reply := frame
	reply.Type = FrameChunkAck
	reply.Payload = nil
	reply.Length = 0
	if !matchesV3Reply(reply, frame, FrameChunkAck) {
		t.Fatal("valid reply rejected")
	}
	changes := []func(*BinaryFrameV3){func(f *BinaryFrameV3) { f.TransferID[0]++ }, func(f *BinaryFrameV3) { f.SessionID[0]++ }, func(f *BinaryFrameV3) { f.Generation++ }, func(f *BinaryFrameV3) { f.StreamID++ }, func(f *BinaryFrameV3) { f.Sequence++ }, func(f *BinaryFrameV3) { f.Offset++ }}
	for _, change := range changes {
		stale := reply
		change(&stale)
		if matchesV3Reply(stale, frame, FrameChunkAck) {
			t.Fatal("stale ACK accepted")
		}
	}
}

func TestV3LegacySendersRejectBeforeIO(t *testing.T) {
	engine := NewEngine()
	if _, err := engine.transferBinaryFilePipelined(context.Background(), "peer", Message{}, nil, nil, nil, transferTuning{}, "dzhgo/3"); err == nil {
		t.Fatal("legacy window accepted")
	}
	if err := engine.transferParallelFile(context.Background(), Peer{}, Message{}, nil, nil, nil, protocolDialects[0], "", 4, "dzhgo/3"); err == nil {
		t.Fatal("legacy parallel accepted")
	}
	for _, kind := range []string{"file_chunk", "file_window", "file_stream_join", "file_complete", "share_chunk"} {
		if !isLegacyFileData(kind) {
			t.Fatal(kind)
		}
	}
	for _, kind := range []string{"file_offer", "file_manifest", "file_thumbnail", "chat"} {
		if isLegacyFileData(kind) {
			t.Fatal(kind)
		}
	}
}

func TestV3CanceledSenderInterruptsBlockedIO(t *testing.T) {
	engine := NewEngine()
	pool := engine.v3Pool("peer", 1)
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	pool.slots[0].SetConnection(a)
	f, err := os.CreateTemp(t.TempDir(), "source")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- engine.sendV3FileData(ctx, Peer{DeviceID: "peer"}, Message{AttachmentID: "file"}, f, "", 0)
	}()
	// Read BeginFile then withhold EndFile acknowledgement.
	if _, err := ReadBinaryFrameV3(b, 1024); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBinaryFrameV3(b, 1024); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled transfer succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not interrupt IO")
	}
	if _, err := b.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("connection still open: %v", err)
	}
}

func TestV3SharedPreviewStreamsRequestedRange(t *testing.T) {
	for _, offset := range []int64{0, 12345} {
		t.Run(fmt.Sprint(offset), func(t *testing.T) {
			payload := bytes.Repeat([]byte("preview"), 400000)
			hash := sha256.Sum256(payload)
			sender, receiver := NewEngine(), NewEngine()
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			pool := sender.v3Pool("peer", 1)
			pool.slots[0].SetConnection(a)
			var received bytes.Buffer
			transfer := &incomingFile{attachmentID: "preview", expected: int64(len(payload)), received: offset, sha256: hex.EncodeToString(hash[:]), v3StartOffset: offset, digest: sha256.New(), v3Done: make(chan string, 1), v3Sink: func(p []byte) error { _, err := received.Write(p); return err }}
			if offset > 0 {
				transfer.v3Ranges = []ByteRange{{0, offset}}
			}
			receiver.incoming["preview"] = transfer
			done := make(chan error, 1)
			go func() { done <- receiver.receiveV3Transfer(b) }()
			path := filepath.Join(t.TempDir(), "preview.src")
			if err := os.WriteFile(path, payload, 0600); err != nil {
				t.Fatal(err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := sender.sendV3FileData(ctx, Peer{DeviceID: "peer"}, Message{AttachmentID: "preview", AttachmentSize: int64(len(payload))}, f, hex.EncodeToString(hash[:]), offset); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(received.Bytes(), payload[offset:]) {
				t.Fatal("preview skipped or resent bytes outside requested range")
			}
		})
	}
}

func TestV3CommitFailurePreservesPartAndReportsFailure(t *testing.T) {
	openSharedFolderTestDatabase(t)
	root := t.TempDir()
	path := filepath.Join(root, "received.part")
	data := []byte("verified data")
	sum := sha256.Sum256(data)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	blocker := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blocker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	engine := NewEngine()
	engine.incoming["commit-test"] = &incomingFile{file: f, tempPath: path, targetPath: filepath.Join(blocker, "file"), attachmentID: "commit-test", expected: int64(len(data)), received: int64(len(data)), sha256: hex.EncodeToString(sum[:]), v3Streams: map[uint16]*v3StreamState{0: {}}, v3Ranges: []ByteRange{{0, int64(len(data))}}}
	if result := engine.finishIncomingFile("commit-test"); result != "failed" {
		t.Fatalf("commit failure reported as %s", result)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("lost recoverable part: %v", err)
	}
}

func TestV3PausePreservesSparseDurableRanges(t *testing.T) {
	openSharedFolderTestDatabase(t)
	path, _, err := transferResumePaths("pause-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(100); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("range"), 50); err != nil {
		t.Fatal(err)
	}
	transfer := &incomingFile{attachmentID: "pause-test", file: f, tempPath: path, expected: 100, received: 5, v3Streams: map[uint16]*v3StreamState{0: {}}, v3Ranges: []ByteRange{{50, 55}}, resumeState: transferResumeState{AttachmentID: "pause-test", FileSize: 100, TempPath: path}}
	engine := NewEngine()
	engine.incoming["pause-test"] = transfer
	engine.pauseIncomingFile("pause-test", "NETWORK_UNSTABLE")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 100 || string(data[50:55]) != "range" {
		t.Fatal("pause truncated acknowledged sparse data")
	}
	state, err := loadTransferResumeState("pause-test")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.CompletedRanges) != 1 || state.CompletedRanges[0].Offset != 50 || state.CompletedRanges[0].Length != 5 {
		t.Fatalf("lost ranges: %+v", state.CompletedRanges)
	}
}
