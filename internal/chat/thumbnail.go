package chat

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"strings"
)

const (
	thumbnailMaxEdge = 640
	thumbnailMaxSize = 128 * 1024
)

// buildImageThumbnail returns a small base64 encoded JPEG for image files.
// Unsupported image formats deliberately fall back to no thumbnail; the
// original attachment can still be transferred normally.
func buildImageThumbnail(path, mimeType string) (string, string, error) {
	if !strings.HasPrefix(mimeType, "image/") {
		return "", "", nil
	}
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	source, _, err := image.Decode(file)
	if err != nil {
		return "", "", err
	}
	thumbnail := resizeThumbnail(source, thumbnailMaxEdge)
	for quality := 90; quality >= 40; quality -= 5 {
		var buffer bytes.Buffer
		if err := jpeg.Encode(&buffer, thumbnail, &jpeg.Options{Quality: quality}); err != nil {
			return "", "", err
		}
		if buffer.Len() <= thumbnailMaxSize || quality == 35 {
			if buffer.Len() > thumbnailMaxSize {
				thumbnail = resizeThumbnail(thumbnail, thumbnailMaxEdge/2)
				buffer.Reset()
				if err := jpeg.Encode(&buffer, thumbnail, &jpeg.Options{Quality: 40}); err != nil {
					return "", "", err
				}
			}
			return base64.StdEncoding.EncodeToString(buffer.Bytes()), "image/jpeg", nil
		}
	}
	return "", "", fmt.Errorf("生成缩略图失败")
}

// buildAvatarPreview creates the small image sent with public discovery
// announces. It is intentionally much smaller than a normal file thumbnail so
// a discovery datagram stays cheap and does not contain the user's original
// avatar bytes.
func buildAvatarPreview(data []byte, mimeType string) (encoded, previewMime string, previewBytes []byte, err error) {
	if !strings.HasPrefix(strings.ToLower(mimeType), "image/") || len(data) == 0 {
		return "", "", nil, nil
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", "", nil, err
	}
	thumbnail := resizeThumbnail(source, 192)
	for quality := 82; quality >= 35; quality -= 7 {
		var buffer bytes.Buffer
		if err := jpeg.Encode(&buffer, thumbnail, &jpeg.Options{Quality: quality}); err != nil {
			return "", "", nil, err
		}
		if buffer.Len() <= 48*1024 || quality == 35 {
			previewBytes = append([]byte(nil), buffer.Bytes()...)
			return base64.StdEncoding.EncodeToString(previewBytes), "image/jpeg", previewBytes, nil
		}
	}
	return "", "", nil, fmt.Errorf("生成头像预览失败")
}

func resizeThumbnail(source image.Image, maxEdge int) *image.RGBA {
	bound := source.Bounds()
	width, height := bound.Dx(), bound.Dy()
	if width <= 0 || height <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	if width > maxEdge || height > maxEdge {
		if width >= height {
			height = height * maxEdge / width
			width = maxEdge
		} else {
			width = width * maxEdge / height
			height = maxEdge
		}
	}
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	target := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		sourceY := bound.Min.Y + y*bound.Dy()/height
		for x := 0; x < width; x++ {
			sourceX := bound.Min.X + x*bound.Dx()/width
			target.Set(x, y, source.At(sourceX, sourceY))
		}
	}
	return target
}
