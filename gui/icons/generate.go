//go:build ignore

package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const (
	size   = 32
	cx     = size / 2.0
	cy     = size / 2.0
	radius = 13.0
)

func main() {
	generateIcon("red.png", color.NRGBA{220, 50, 50, 255})
	generateIcon("green.png", color.NRGBA{50, 180, 50, 255})
}

func generateIcon(filename string, fill color.NRGBA) {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			dist := math.Sqrt(dx*dx + dy*dy)

			if dist <= radius-1.0 {
				// Inner fill
				img.SetNRGBA(x, y, fill)
			} else if dist <= radius {
				// Anti-aliased edge
				alpha := radius - dist // 0..1
				img.SetNRGBA(x, y, color.NRGBA{fill.R, fill.G, fill.B, uint8(alpha * 255)})
			} else if dist <= radius+1.0 {
				// Slight shadow/border just outside the circle
				alpha := (radius + 1.0 - dist) * 0.3
				img.SetNRGBA(x, y, color.NRGBA{0, 0, 0, uint8(alpha * 255)})
			}
		}
	}

	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}
