package chat

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"net"
	"testing"
	"time"
)

func TestV3RangeResume(t *testing.T) {
	got := MergeRanges([]ByteRange{{10, 20}, {0, 10}, {18, 30}})
	if len(got) != 1 || got[0].Start != 0 || got[0].End != 30 {
		t.Fatalf("%v", got)
	}
	miss := MissingRanges(50, got)
	if len(miss) != 1 || miss[0].Start != 30 {
		t.Fatalf("%v", miss)
	}
}

func TestV3PoolKeepsConnectionAcrossReleasesAndClosesIdle(t *testing.T) {
	p := NewPeerPool("peer", 1)
	a, b := net.Pipe()
	defer b.Close()
	s, err := p.Acquire(context.Background(), "first")
	if err != nil {
		t.Fatal(err)
	}
	s.SetConnection(a)
	p.Release(s)
	s2, err := p.Acquire(context.Background(), "second")
	if err != nil || s2.Connection() != a {
		t.Fatalf("pooled connection was not reused: err=%v same=%v", err, err == nil && s2.Connection() == a)
	}
	p.Release(s2)
	s.LastProgress = time.Now().Add(-time.Minute)
	p.CloseIdle(time.Now(), time.Second)
	if s.Connection() != nil {
		t.Fatal("idle connection was not closed")
	}
}

func TestV3FrameSessionGenerationRoundTrip(t *testing.T) {
	var session [16]byte
	for i := range session {
		session[i] = byte(i + 1)
	}
	f := NewChunkFrame([16]byte{9}, 2, 7, 128, []byte("payload"))
	f.SessionID, f.Generation = session, 42
	raw, err := f.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ReadBinaryFrameV3(bytes.NewReader(raw), 1024)
	if err != nil || decoded.SessionID != session || decoded.Generation != 42 || string(decoded.Payload) != "payload" {
		t.Fatalf("frame metadata mismatch: err=%v decoded=%+v", err, decoded)
	}
}

type oneByteReader struct {
	r io.Reader
}

func (r oneByteReader) Read(dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}
	return r.r.Read(dst[:1])
}

func TestV3FrameReaderSurvivesFragmentedSlowPayload(t *testing.T) {
	frame := NewChunkFrame([16]byte{7}, 1, 2, 3, bytes.Repeat([]byte("x"), 64*1024))
	raw, err := frame.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ReadBinaryFrameV3(oneByteReader{r: bytes.NewReader(raw)}, uint32(len(frame.Payload)))
	if err != nil || len(decoded.Payload) != len(frame.Payload) || decoded.Offset != frame.Offset {
		t.Fatalf("fragmented frame failed: err=%v length=%d", err, len(decoded.Payload))
	}
}

func TestV3WatchdogKeepsSlotWithRecentIOAndDelayedAck(t *testing.T) {
	p := NewPeerPool("peer", 1)
	s, err := p.Acquire(context.Background(), "attachment")
	if err != nil {
		t.Fatal(err)
	}
	a, b := net.Pipe()
	defer b.Close()
	s.SetConnection(a)
	s.MarkSent(1024)
	s.TouchIO("write")
	if got := p.Watchdog(time.Now().Add(1500 * time.Millisecond)); got != 0 {
		t.Fatalf("active slot was killed before its I/O became idle: %d", got)
	}
	s.TouchIO("read")
	if got := p.Watchdog(time.Now().Add(1500 * time.Millisecond)); got != 0 {
		t.Fatalf("slot with recent I/O was killed while ACK was delayed: %d", got)
	}
	p.Release(s)
}

func TestV3WatchdogAllowsSlowDurabilityCheckpoint(t *testing.T) {
	s := &PoolSlot{State: SlotTransferring, currentTransferID: "attachment"}
	now := time.Now()
	s.MarkSent(1024)
	if !s.MarkProbe(now) {
		t.Fatal("probe was not recorded")
	}
	if s.ProbeExpired(now.Add(9 * time.Second)) {
		t.Fatal("slot expired during the slow durability window")
	}
	if !s.ProbeExpired(now.Add(10 * time.Second)) {
		t.Fatal("slot did not expire after a full probe timeout")
	}
}

func TestV3WatchdogProbeRemainsPendingAfterPingWrite(t *testing.T) {
	p := NewPeerPool("peer", 1)
	s, err := p.Acquire(context.Background(), "attachment")
	if err != nil {
		t.Fatal(err)
	}
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	s.SetConnection(a)
	s.MarkSent(1)
	now := time.Now()

	readDone := make(chan error, 1)
	go func() {
		_, err := ReadBinaryFrameV3(b, 1024)
		readDone <- err
	}()
	if got := p.Watchdog(now.Add(3 * time.Second)); got != 0 {
		t.Fatalf("probe write unexpectedly killed slot: %d", got)
	}
	if err := <-readDone; err != nil {
		t.Fatal(err)
	}
	if got := p.Watchdog(now.Add(12 * time.Second)); got != 0 {
		t.Fatalf("pending probe expired during slow durability: %d", got)
	}
	if got := p.Watchdog(now.Add(13 * time.Second)); got != 1 {
		t.Fatalf("dead slot was not isolated after probe timeout: %d", got)
	}
}

func TestV3FrameReaderRepliesToPingWithoutDroppingControlFrame(t *testing.T) {
	a, b := net.Pipe()
	reader := newV3FrameReader(a, 1024)
	defer reader.Close()
	defer b.Close()

	ping := BinaryFrameV3{Type: FramePoolPing, TransferID: [16]byte{1}, StreamID: 2, Sequence: 3, SessionID: [16]byte{4}, Generation: 5}
	writeDone := make(chan error, 1)
	go func() { writeDone <- writeV3Frame(b, ping) }()
	pong, err := ReadBinaryFrameV3(b, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if pong.Type != FramePoolPong || pong.TransferID != ping.TransferID || pong.SessionID != ping.SessionID || pong.Generation != ping.Generation {
		t.Fatalf("unexpected pong: %+v", pong)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	queued, err := reader.Next()
	if err != nil || queued.Type != FramePoolPing {
		t.Fatalf("ping was not retained for batch delimiting: frame=%+v err=%v", queued, err)
	}
}

func TestV3SlotIsolation(t *testing.T) {
	s := &PoolSlot{State: SlotIdle, LastProgress: time.Now()}
	if e := s.Reserve("x"); e != nil || !s.Start() {
		t.Fatal("reserve")
	}
	s.Kill()
	s.Release()
	if s.State != SlotDead {
		t.Fatal("dead slot released")
	}
}

func TestV3ManifestPathValidation(t *testing.T) {
	root := t.TempDir()
	if _, err := ValidateManifestPath(root, "../escape"); err == nil {
		t.Fatal("path traversal accepted")
	}
	if _, err := ValidateManifestPath(root, "/absolute"); err == nil {
		t.Fatal("absolute path accepted")
	}
	if _, err := ValidateManifestPath(root, "folder/file.bin"); err != nil {
		t.Fatal(err)
	}
}

func TestV3RangeWriterIdempotentAndVerified(t *testing.T) {
	root := t.TempDir()
	final := root + "/file.bin"
	w, err := NewRangeWriterV3(final, 6)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	a := []byte("abc")
	h := sha256.Sum256(a)
	ok, err := w.WriteChunk(0, a, h[:])
	if err != nil || !ok {
		t.Fatal(err)
	}
	ok, err = w.WriteChunk(0, a, h[:])
	if err != nil || !ok {
		t.Fatal(err)
	}
	b := []byte("def")
	hb := sha256.Sum256(b)
	if _, err = w.WriteChunk(3, b, hb[:]); err != nil {
		t.Fatal(err)
	}
	whole := sha256.Sum256([]byte("abcdef"))
	if err = w.Complete(whole[:]); err != nil {
		t.Fatal(err)
	}
}
