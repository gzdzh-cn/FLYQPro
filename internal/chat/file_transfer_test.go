package chat

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

func TestRequiredAttachmentBytesAddsSafetyMargin(t *testing.T) {
	if got, want := requiredAttachmentBytes(100), int64(100)+attachmentSafetyMargin; got != want {
		t.Fatalf("required bytes = %d, want %d", got, want)
	}
	if got := requiredAttachmentBytes(-1); got <= 0 {
		t.Fatalf("negative file size should remain rejectable, got %d", got)
	}
}

func TestBinaryFileFrameHeaderRoundTrip(t *testing.T) {
	input := binaryFileFrameHeader{WindowID: 9, StartChunk: 24, ChunkCount: 8, ChunkSize: 512 * 1024, PayloadLen: 4 * 1024 * 1024}
	var encoded bytes.Buffer
	if err := writeBinaryFileFrameHeader(&encoded, input); err != nil {
		t.Fatal(err)
	}
	output, err := readBinaryFileFrameHeader(&encoded)
	if err != nil {
		t.Fatal(err)
	}
	if output != input {
		t.Fatalf("binary frame header = %+v, want %+v", output, input)
	}
}

func TestBinaryFileFrameHeaderRejectsInvalidAndTruncatedFrames(t *testing.T) {
	input := binaryFileFrameHeader{WindowID: 1, StartChunk: 0, ChunkCount: 1, ChunkSize: 256 * 1024, PayloadLen: 256 * 1024}
	var encoded bytes.Buffer
	if err := writeBinaryFileFrameHeader(&encoded, input); err != nil {
		t.Fatal(err)
	}
	data := encoded.Bytes()
	data[0] = 'X'
	if _, err := readBinaryFileFrameHeader(bytes.NewReader(data)); err == nil {
		t.Fatal("invalid magic should be rejected")
	}
	if _, err := readBinaryFileFrameHeader(io.LimitReader(bytes.NewReader(data), binaryFileFrameHeaderSize-1)); err == nil {
		t.Fatal("truncated frame header should be rejected")
	}
}

func TestWireReaderPreservesBinaryBytesAfterControlFrame(t *testing.T) {
	control, err := json.Marshal(wireMessage{Type: "file_window", AttachmentID: "attachment", TransferMode: binaryTransferMode})
	if err != nil {
		t.Fatal(err)
	}
	header := binaryFileFrameHeader{WindowID: 2, StartChunk: 4, ChunkCount: 1, ChunkSize: 256 * 1024, PayloadLen: 3}
	var stream bytes.Buffer
	stream.Write(control)
	stream.WriteByte('\n')
	if err := writeBinaryFileFrameHeader(&stream, header); err != nil {
		t.Fatal(err)
	}
	stream.WriteString("abc")

	reader := newWireReader(&stream)
	var message wireMessage
	if err := reader.Decode(&message); err != nil {
		t.Fatal(err)
	}
	if message.Type != "file_window" || message.TransferMode != binaryTransferMode {
		t.Fatalf("unexpected control frame: %+v", message)
	}
	decodedHeader, err := readBinaryFileFrameHeader(reader.reader)
	if err != nil {
		t.Fatal(err)
	}
	if decodedHeader != header {
		t.Fatalf("binary header = %+v, want %+v", decodedHeader, header)
	}
	payload, err := io.ReadAll(io.LimitReader(reader.reader, int64(decodedHeader.PayloadLen)))
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "abc" {
		t.Fatalf("binary payload = %q, want abc", payload)
	}
}

func TestBinaryFileWindowStreamsRawPayload(t *testing.T) {
	payload := bytes.Repeat([]byte("flyqpro-binary-window"), 160000)
	path := t.TempDir() + "/payload.bin"
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	sender, receiver := net.Pipe()
	defer sender.Close()
	defer receiver.Close()
	result := make(chan error, 1)
	expectedChunks := (len(payload) + minTransferChunkSize - 1) / minTransferChunkSize
	go func() {
		chunks, written, writeErr := newWireSession(sender).writeBinaryFileWindow(file, "attachment", 3, 12, minTransferChunkSize, 16, int64(len(payload)))
		if writeErr == nil && (chunks != expectedChunks || written != int64(len(payload))) {
			writeErr = io.ErrShortWrite
		}
		result <- writeErr
	}()

	reader := newWireReader(receiver)
	var control wireMessage
	if err := reader.Decode(&control); err != nil {
		t.Fatal(err)
	}
	if control.Type != "file_window" || control.TransferMode != binaryTransferMode || control.WindowBytes != int64(len(payload)) {
		t.Fatalf("unexpected binary window control: %+v", control)
	}
	header, err := readBinaryFileFrameHeader(reader.reader)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, header.PayloadLen)
	if _, err := io.ReadFull(reader.reader, got); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("binary window payload changed during streaming")
	}
}

func TestReceiveBinaryFileWindowWritesAndAcknowledges(t *testing.T) {
	payload := bytes.Repeat([]byte("receive-binary-window"), 50000)
	file, err := os.Create(t.TempDir() + "/received.bin")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()

	header := binaryFileFrameHeader{WindowID: 2, StartChunk: 7, ChunkCount: uint32((len(payload) + minTransferChunkSize - 1) / minTransferChunkSize), ChunkSize: minTransferChunkSize, PayloadLen: uint64(len(payload))}
	var stream bytes.Buffer
	if err := writeBinaryFileFrameHeader(&stream, header); err != nil {
		t.Fatal(err)
	}
	stream.Write(payload)
	digest := sha256.New()
	transfer := &incomingFile{file: file, writer: bufio.NewWriterSize(file, 1024*1024), attachmentID: "attachment", messageID: "message", senderID: "peer", expected: int64(len(payload)), digest: digest, session: newWireSession(local), windowed: true, binary: true, binaryPending: true, windowID: int(header.WindowID), nextWindowID: int(header.WindowID) + 1, windowSize: int(header.ChunkCount), nextChunk: int(header.StartChunk), chunkSize: int(header.ChunkSize), expectedWindowBytes: int64(len(payload))}

	ackResult := make(chan wireMessage, 1)
	go func() {
		var ack wireMessage
		_ = json.NewDecoder(remote).Decode(&ack)
		ackResult <- ack
	}()
	if err := NewEngine().receiveBinaryFileWindow(newWireReader(&stream), transfer); err != nil {
		t.Fatal(err)
	}
	ack := <-ackResult
	if ack.Type != "file_progress" || ack.Transferred != int64(len(payload)) || ack.TransferMode != binaryTransferMode {
		t.Fatalf("unexpected binary window acknowledgement: %+v", ack)
	}
	if transfer.received != int64(len(payload)) || transfer.nextChunk != int(header.StartChunk+header.ChunkCount) || transfer.binaryPending {
		t.Fatalf("unexpected receiver state: %+v", transfer)
	}
	if got, want := digest.Sum(nil), sha256.Sum256(payload); !bytes.Equal(got, want[:]) {
		t.Fatal("received binary payload SHA-256 mismatch")
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("received binary payload changed on disk")
	}
}

func TestJSONFileWindowCompatibilityPath(t *testing.T) {
	payload := bytes.Repeat([]byte("legacy-window"), 90000)
	path := t.TempDir() + "/payload.bin"
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	sender, receiver := net.Pipe()
	defer sender.Close()
	defer receiver.Close()
	result := make(chan error, 1)
	go func() {
		_, _, writeErr := newWireSession(sender).writeFileWindow(file, "attachment", 1, 0, minTransferChunkSize, 16)
		result <- writeErr
	}()

	decoder := json.NewDecoder(receiver)
	var control wireMessage
	if err := decoder.Decode(&control); err != nil {
		t.Fatal(err)
	}
	if control.Type != "file_window" || control.TransferMode != jsonWindowTransferMode {
		t.Fatalf("unexpected JSON window control: %+v", control)
	}
	var got bytes.Buffer
	for got.Len() < len(payload) {
		var chunk wireMessage
		if err := decoder.Decode(&chunk); err != nil {
			t.Fatal(err)
		}
		decoded, err := base64.StdEncoding.DecodeString(chunk.Payload)
		if err != nil {
			t.Fatal(err)
		}
		got.Write(decoded)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), payload) {
		t.Fatal("JSON compatibility payload changed during streaming")
	}
}

func TestCancelInterruptsBinaryFileWindow(t *testing.T) {
	path := t.TempDir() + "/payload.bin"
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(8 * 1024 * 1024); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	sender, receiver := net.Pipe()
	defer receiver.Close()
	session := newWireSession(sender)
	result := make(chan error, 1)
	go func() {
		_, _, writeErr := session.writeBinaryFileWindow(file, "attachment", 0, 0, maxTransferChunkSize, 8, 8*1024*1024)
		result <- writeErr
	}()
	time.Sleep(10 * time.Millisecond)
	session.cancel("attachment")
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("canceled binary window unexpectedly completed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not interrupt binary window")
	}
}

func BenchmarkFileWindowTransportEncoding(b *testing.B) {
	const payloadSize = int64(64 * 1024 * 1024)
	path := b.TempDir() + "/payload.bin"
	file, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	if err := file.Truncate(payloadSize); err != nil {
		b.Fatal(err)
	}
	if err := file.Close(); err != nil {
		b.Fatal(err)
	}

	b.Run("binary-window", func(b *testing.B) {
		b.SetBytes(payloadSize)
		for index := 0; index < b.N; index++ {
			benchmarkBinaryFileWindow(b, path, payloadSize)
		}
	})
	b.Run("json-window", func(b *testing.B) {
		b.SetBytes(payloadSize)
		for index := 0; index < b.N; index++ {
			benchmarkJSONFileWindow(b, path, payloadSize)
		}
	})
}

func benchmarkBinaryFileWindow(b *testing.B, path string, payloadSize int64) {
	file, err := os.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()
	sender, receiver := net.Pipe()
	defer sender.Close()
	defer receiver.Close()
	result := make(chan error, 1)
	go func() {
		_, _, writeErr := newWireSession(sender).writeBinaryFileWindow(file, "attachment", 0, 0, maxTransferChunkSize, 64, payloadSize)
		result <- writeErr
	}()
	reader := newWireReader(receiver)
	var control wireMessage
	if err := reader.Decode(&control); err != nil {
		b.Fatal(err)
	}
	header, err := readBinaryFileFrameHeader(reader.reader)
	if err != nil {
		b.Fatal(err)
	}
	if _, err := io.CopyN(io.Discard, reader.reader, int64(header.PayloadLen)); err != nil {
		b.Fatal(err)
	}
	if err := <-result; err != nil {
		b.Fatal(err)
	}
}

func benchmarkJSONFileWindow(b *testing.B, path string, payloadSize int64) {
	file, err := os.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()
	sender, receiver := net.Pipe()
	defer sender.Close()
	defer receiver.Close()
	result := make(chan error, 1)
	go func() {
		_, _, writeErr := newWireSession(sender).writeFileWindow(file, "attachment", 0, 0, maxTransferChunkSize, int(payloadSize/int64(maxTransferChunkSize)))
		result <- writeErr
	}()
	decoder := json.NewDecoder(receiver)
	var control wireMessage
	if err := decoder.Decode(&control); err != nil {
		b.Fatal(err)
	}
	var received int64
	for received < payloadSize {
		var chunk wireMessage
		if err := decoder.Decode(&chunk); err != nil {
			b.Fatal(err)
		}
		data, err := base64.StdEncoding.DecodeString(chunk.Payload)
		if err != nil {
			b.Fatal(err)
		}
		received += int64(len(data))
	}
	if err := <-result; err != nil {
		b.Fatal(err)
	}
}

func TestFileOfferResponseWireFieldsArePortable(t *testing.T) {
	input := wireMessage{Type: "file_offer_response", AttachmentID: "a", Status: "rejected", Reason: "INSUFFICIENT_STORAGE", AvailableBytes: 12, RequiredBytes: 34}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output wireMessage
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Reason != input.Reason || output.AvailableBytes != input.AvailableBytes || output.RequiredBytes != input.RequiredBytes {
		t.Fatalf("wire response fields did not round-trip: %+v", output)
	}
}

func TestFileWindowWireFieldsRoundTrip(t *testing.T) {
	input := wireMessage{Type: "file_window", AttachmentID: "a", ChunkIndex: 8, ChunkSize: 256 * 1024, WindowID: 3, WindowSize: 16, WindowBytes: 4 * 1024 * 1024}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output wireMessage
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Type != input.Type || output.AttachmentID != input.AttachmentID || output.ChunkSize != input.ChunkSize || output.WindowID != input.WindowID || output.WindowSize != input.WindowSize || output.WindowBytes != input.WindowBytes {
		t.Fatalf("file window fields did not round-trip: %+v", output)
	}
}
