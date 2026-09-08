package chat

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"
)

func TestReceiveBinaryFileV3(t *testing.T) {
	root := t.TempDir()
	path := root + "/x"
	w, e := NewRangeWriterV3(path, 6)
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	id := [16]byte{1}
	a := []byte("abc")
	b := []byte("def")
	fa := NewChunkFrame(id, 0, 0, 0, a)
	fb := NewChunkFrame(id, 0, 1, 3, b)
	whole := sha256.Sum256([]byte("abcdef"))
	end := BinaryFrameV3{Type: FrameEndFile, TransferID: id, ChunkHash: whole}
	var data bytes.Buffer
	for _, f := range []BinaryFrameV3{fa, fb, end} {
		raw, _ := f.MarshalBinary()
		data.Write(raw)
	}
	got := ReceiveBinaryFileV3(context.Background(), &data, w, 1024, nil)
	if got.Status != "completed" || got.Completed != 6 {
		t.Fatalf("%+v", got)
	}
}
