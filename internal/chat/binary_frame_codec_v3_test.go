package chat

import (
	"bytes"
	"encoding/hex"
	"io"
	"testing"
)

// Identical golden packet is consumed by the Android codec tests.
const v3GoldenChunk = "445a4833000303000102030405060708090a0b0c0d0e0f12340102030405060708000000000000100000000003ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad101112131415161718191a1b1c1d1e1f000000000000002a616263"

func TestV3CodecGoldenPacket(t *testing.T) {
	raw, _ := hex.DecodeString(v3GoldenChunk)
	f, err := ReadBinaryFrameV3(bytes.NewReader(raw), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if f.Type != FrameChunkData || f.StreamID != 0x1234 || f.Sequence != 0x102030405060708 || f.Offset != 4096 || f.Generation != 42 || string(f.Payload) != "abc" {
		t.Fatalf("wrong fields: %+v", f)
	}
	encoded, err := f.MarshalBinary()
	if err != nil || !bytes.Equal(encoded, raw) {
		t.Fatalf("golden mismatch: %v", err)
	}
}

func TestV3CodecConsecutiveControlFrames(t *testing.T) {
	var wire bytes.Buffer
	for typ := FrameBeginFile; typ <= FramePoolPong; typ++ {
		f := BinaryFrameV3{Type: typ, TransferID: [16]byte{9}, SessionID: [16]byte{4}, Generation: 42}
		raw, err := f.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		wire.Write(raw)
	}
	for typ := FrameBeginFile; typ <= FramePoolPong; typ++ {
		f, err := ReadBinaryFrameV3(&wire, 1024)
		if err != nil || f.Type != typ || f.TransferID[0] != 9 || f.SessionID[0] != 4 || f.Generation != 42 {
			t.Fatalf("type=%d frame=%+v err=%v", typ, f, err)
		}
	}
	if _, err := ReadBinaryFrameV3(&wire, 1024); err != io.EOF {
		t.Fatalf("unexpected trailing data: %v", err)
	}
}

func TestV3CodecRejectsInvalidHeader(t *testing.T) {
	for typ := FrameBeginFile; typ <= FramePoolPong; typ++ {
		if _, err := (BinaryFrameV3{Type: typ, Length: 1024}).MarshalBinary(); err == nil {
			t.Fatalf("type %d accepts phantom payload", typ)
		}
	}
	raw, _ := hex.DecodeString(v3GoldenChunk)
	for _, code := range []byte{0, 12, 255} {
		raw[6] = code
		if _, err := ReadBinaryFrameV3(bytes.NewReader(raw), 1024); err == nil {
			t.Fatal("unknown type accepted")
		}
		if _, err := (BinaryFrameV3{Type: V3FrameType(code)}).MarshalBinary(); err == nil {
			t.Fatal("unknown type encoded")
		}
	}
}
