// Command icongen turns the original FlyQPro artwork into a platform app icon.
//
// The source artwork has a white background. We remove only the white area that
// is connected to the canvas edge, preserving the white speech bubble inside
// the logo, then place the mark on a padded rounded-square tile.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	xdraw "golang.org/x/image/draw"
)

const (
	canvasSize    = 1024
	tileInset     = 104
	tileRadius    = 190
	markMaxWidth  = 620
	markMaxHeight = 620
)

func main() {
	input := flag.String("input", "icon-source.png", "source artwork")
	output := flag.String("output", "appicon.png", "generated PNG")
	flag.Parse()

	source, err := readPNG(*input)
	if err != nil {
		fatal(err)
	}
	mark := removeEdgeBackground(source)
	bounds := alphaBounds(mark)
	if bounds.Empty() {
		fatal(fmt.Errorf("source contains no visible artwork"))
	}

	markWidth, markHeight := fittedSize(bounds.Dx(), bounds.Dy(), markMaxWidth, markMaxHeight)
	scaled := image.NewNRGBA(image.Rect(0, 0, markWidth, markHeight))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), mark, bounds, xdraw.Over, nil)

	icon := image.NewNRGBA(image.Rect(0, 0, canvasSize, canvasSize))
	drawTile(icon)
	markX := (canvasSize - markWidth) / 2
	markY := (canvasSize-markHeight)/2 - 4
	xdraw.Draw(icon, image.Rect(markX, markY, markX+markWidth, markY+markHeight), scaled, image.Point{}, xdraw.Over)

	if err := writePNG(*output, icon); err != nil {
		fatal(err)
	}
}

func readPNG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return png.Decode(file)
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	return encoder.Encode(file, img)
}

func removeEdgeBackground(source image.Image) *image.NRGBA {
	bounds := source.Bounds()
	result := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			result.Set(x, y, source.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}

	visited := make([]bool, bounds.Dx()*bounds.Dy())
	queue := make([]image.Point, 0, 2*(bounds.Dx()+bounds.Dy()))
	push := func(x, y int) {
		if x < 0 || y < 0 || x >= bounds.Dx() || y >= bounds.Dy() {
			return
		}
		index := y*bounds.Dx() + x
		if visited[index] || !isBackground(result.NRGBAAt(x, y)) {
			return
		}
		visited[index] = true
		queue = append(queue, image.Pt(x, y))
	}
	for x := 0; x < bounds.Dx(); x++ {
		push(x, 0)
		push(x, bounds.Dy()-1)
	}
	for y := 0; y < bounds.Dy(); y++ {
		push(0, y)
		push(bounds.Dx()-1, y)
	}
	for head := 0; head < len(queue); head++ {
		point := queue[head]
		result.SetNRGBA(point.X, point.Y, color.NRGBA{})
		push(point.X-1, point.Y)
		push(point.X+1, point.Y)
		push(point.X, point.Y-1)
		push(point.X, point.Y+1)
	}
	return result
}

func isBackground(pixel color.NRGBA) bool {
	return pixel.A == 0 || (pixel.R >= 246 && pixel.G >= 246 && pixel.B >= 246)
}

func alphaBounds(img *image.NRGBA) image.Rectangle {
	minX, minY := img.Bounds().Max.X, img.Bounds().Max.Y
	maxX, maxY := 0, 0
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			if img.NRGBAAt(x, y).A < 8 {
				continue
			}
			minX = min(minX, x)
			minY = min(minY, y)
			maxX = max(maxX, x+1)
			maxY = max(maxY, y+1)
		}
	}
	return image.Rect(minX, minY, maxX, maxY)
}

func fittedSize(width, height, maxWidth, maxHeight int) (int, int) {
	scale := math.Min(float64(maxWidth)/float64(width), float64(maxHeight)/float64(height))
	return int(math.Round(float64(width) * scale)), int(math.Round(float64(height) * scale))
}

func drawTile(img *image.NRGBA) {
	left, top := float64(tileInset), float64(tileInset)
	right, bottom := float64(canvasSize-tileInset), float64(canvasSize-tileInset)
	for y := tileInset - 1; y <= canvasSize-tileInset; y++ {
		progress := float64(y-tileInset) / float64(canvasSize-2*tileInset)
		progress = math.Max(0, math.Min(1, progress))
		fill := color.NRGBA{
			R: uint8(255 - 10*progress),
			G: uint8(255 - 3*progress),
			B: uint8(255 - 7*progress),
			A: 255,
		}
		for x := tileInset - 1; x <= canvasSize-tileInset; x++ {
			coverage := roundedRectCoverage(float64(x)+0.5, float64(y)+0.5, left, top, right, bottom, tileRadius)
			if coverage <= 0 {
				continue
			}
			pixel := fill
			pixel.A = uint8(math.Round(255 * coverage))
			img.SetNRGBA(x, y, pixel)
		}
	}
}

func roundedRectCoverage(x, y, left, top, right, bottom float64, radius int) float64 {
	r := float64(radius)
	nearestX := math.Max(left+r, math.Min(right-r, x))
	nearestY := math.Max(top+r, math.Min(bottom-r, y))
	distance := math.Hypot(x-nearestX, y-nearestY)
	// A one-pixel transition gives the high-resolution PNG a clean antialiased edge.
	return math.Max(0, math.Min(1, r+0.5-distance))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "icongen:", err)
	os.Exit(1)
}
