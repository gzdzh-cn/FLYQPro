package chat

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const v3FrameMagic uint32 = 0x445A4833
const v3FrameHeaderSize = 4 + 2 + 1 + 16 + 2 + 8 + 8 + 4 + 32 + 16 + 8

type V3FrameType byte

const (
	FrameBeginFile V3FrameType = iota + 1
	FrameChunkMeta
	FrameChunkData
	FrameChunkAck
	FrameChunkNack
	FrameEndFile
	FrameCancel
	FramePause
	FrameResume
	FramePoolPing
	FramePoolPong
)

type BinaryFrameV3 struct {
	Type             V3FrameType
	TransferID       [16]byte
	StreamID         uint16
	Sequence, Offset uint64
	Length           uint32
	ChunkHash        [32]byte
	SessionID        [16]byte
	Generation       uint64
	Payload          []byte
}

func (f BinaryFrameV3) MarshalBinary() ([]byte, error) {
	if f.Type < FrameBeginFile || f.Type > FramePoolPong {
		return nil, errors.New("unknown v3 frame type")
	}
	if uint64(f.Length) != uint64(len(f.Payload)) {
		return nil, errors.New("v3 frame payload length mismatch")
	}
	b := make([]byte, v3FrameHeaderSize+len(f.Payload))
	binary.BigEndian.PutUint32(b, v3FrameMagic)
	binary.BigEndian.PutUint16(b[4:], ProtocolMajor)
	b[6] = byte(f.Type)
	copy(b[7:], f.TransferID[:])
	binary.BigEndian.PutUint16(b[23:], f.StreamID)
	binary.BigEndian.PutUint64(b[25:], f.Sequence)
	binary.BigEndian.PutUint64(b[33:], f.Offset)
	binary.BigEndian.PutUint32(b[41:], f.Length)
	copy(b[45:], f.ChunkHash[:])
	copy(b[77:], f.SessionID[:])
	binary.BigEndian.PutUint64(b[93:], f.Generation)
	copy(b[v3FrameHeaderSize:], f.Payload)
	return b, nil
}

func ReadBinaryFrameV3(r io.Reader, maxPayload uint32) (BinaryFrameV3, error) {
	var h [v3FrameHeaderSize]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return BinaryFrameV3{}, err
	}
	if binary.BigEndian.Uint32(h[:]) != v3FrameMagic {
		return BinaryFrameV3{}, errors.New("invalid v3 frame magic")
	}
	if binary.BigEndian.Uint16(h[4:]) != ProtocolMajor {
		return BinaryFrameV3{}, fmt.Errorf("unsupported v3 frame version")
	}
	if V3FrameType(h[6]) < FrameBeginFile || V3FrameType(h[6]) > FramePoolPong {
		return BinaryFrameV3{}, errors.New("unknown v3 frame type")
	}
	n := binary.BigEndian.Uint32(h[41:])
	if n > maxPayload {
		return BinaryFrameV3{}, errors.New("v3 frame too large")
	}
	f := BinaryFrameV3{Type: V3FrameType(h[6]), StreamID: binary.BigEndian.Uint16(h[23:]), Sequence: binary.BigEndian.Uint64(h[25:]), Offset: binary.BigEndian.Uint64(h[33:]), Length: n}
	copy(f.TransferID[:], h[7:23])
	copy(f.ChunkHash[:], h[45:])
	copy(f.SessionID[:], h[77:93])
	f.Generation = binary.BigEndian.Uint64(h[93:])
	if n > 0 {
		f.Payload = make([]byte, n)
		if _, err := io.ReadFull(r, f.Payload); err != nil {
			return BinaryFrameV3{}, err
		}
	}
	return f, nil
}

func NewChunkFrame(id [16]byte, stream uint16, seq, offset uint64, payload []byte) BinaryFrameV3 {
	return BinaryFrameV3{Type: FrameChunkData, TransferID: id, StreamID: stream, Sequence: seq, Offset: offset, Length: uint32(len(payload)), ChunkHash: sha256.Sum256(payload), Payload: payload}
}

// Control JSON never carries file bytes in v3.
func isLegacyFileData(kind string) bool {
	switch kind {
	case "file_chunk", "file_window", "file_stream_join", "file_complete", "share_chunk":
		return true
	}
	return false
}
