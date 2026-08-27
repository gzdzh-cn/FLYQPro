package service

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestSharedThumbnailStoreReusesAndPrunesCache(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "photo.png")
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	imageData := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			imageData.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 20), B: 180, A: 255})
		}
	}
	if err := png.Encode(file, imageData); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	store := &sharedThumbnailStore{root: filepath.Join(root, "cache"), records: make(map[string]sharedThumbnailRecord), inflight: make(map[string]chan sharedThumbnailResult)}
	first, firstMime, err := store.getOrCreate(root, "photo.png", "image/png", info)
	if err != nil {
		t.Fatal(err)
	}
	if firstMime != "image/jpeg" {
		t.Fatalf("thumbnail mime = %q, want image/jpeg", firstMime)
	}
	second, _, err := store.getOrCreate(root, "photo.png", "image/png", info)
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Fatalf("cache path changed: %q != %q", second, first)
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	store.prune()
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("thumbnail was not pruned, stat error = %v", err)
	}
}
