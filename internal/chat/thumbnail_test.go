package chat

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

func TestBuildImageThumbnailResizesAndLimitsPayload(t *testing.T) {
	path := t.TempDir() + "/source.png"
	source := image.NewRGBA(image.Rect(0, 0, 900, 450))
	for y := 0; y < 450; y++ {
		for x := 0; x < 900; x++ {
			source.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 120, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, source); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	data, mimeType, err := buildImageThumbnail(path, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "image/jpeg" || data == "" {
		t.Fatalf("thumbnail = (%q, %v), want JPEG data", mimeType, data != "")
	}
	encoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > thumbnailMaxSize {
		t.Fatalf("thumbnail size = %d, want <= %d", len(encoded), thumbnailMaxSize)
	}
	decoded, _, err := image.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() > thumbnailMaxEdge || decoded.Bounds().Dy() > thumbnailMaxEdge {
		t.Fatalf("thumbnail dimensions = %v, want longest edge <= %d", decoded.Bounds(), thumbnailMaxEdge)
	}
}

func TestValidThumbnailRejectsOversizedPayload(t *testing.T) {
	data, mimeType := validThumbnail(base64.StdEncoding.EncodeToString(make([]byte, thumbnailMaxSize+1)), "image/jpeg")
	if data != "" || mimeType != "" {
		t.Fatalf("oversized thumbnail was accepted")
	}
}
