//go:build ignore

// Generates tray icons: app icon (32x32) with small red X or green checkmark
// in the bottom-right corner.
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const traySize = 32

func main() {
	generateTrayIcon("red.png", drawRedX)
	generateTrayIcon("green.png", drawGreenCheck)
}

func generateTrayIcon(filename string, drawIndicator func(*image.NRGBA)) {
	img := image.NewNRGBA(image.Rect(0, 0, traySize, traySize))

	// Draw scaled-down app icon: blue circle with arrows
	center := float64(traySize) / 2
	bgRadius := float64(traySize)/2 - 1
	bgColor := color.NRGBA{50, 120, 200, 255}

	for y := 0; y < traySize; y++ {
		for x := 0; x < traySize; x++ {
			dx := float64(x) + 0.5 - center
			dy := float64(y) + 0.5 - center
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist <= bgRadius {
				t := float64(y) / float64(traySize)
				r := uint8(float64(bgColor.R) * (1 - t*0.3))
				g := uint8(float64(bgColor.G) * (1 - t*0.3))
				b := uint8(float64(bgColor.B) * (1 - t*0.3))
				if dist > bgRadius-1 {
					alpha := uint8(255 * (bgRadius - dist))
					img.SetNRGBA(x, y, color.NRGBA{r, g, b, alpha})
				} else {
					img.SetNRGBA(x, y, color.NRGBA{r, g, b, 255})
				}
			}
		}
	}

	// Draw small white arrows
	white := color.NRGBA{255, 255, 255, 255}
	// Right arrow (top half) →
	drawLine(img, 7, 11, 24, 11, 2, white)
	drawLine(img, 21, 8, 25, 11, 2, white)
	drawLine(img, 21, 14, 25, 11, 2, white)
	// Left arrow (bottom half) ←
	drawLine(img, 7, 21, 24, 21, 2, white)
	drawLine(img, 7, 21, 11, 18, 2, white)
	drawLine(img, 7, 21, 11, 24, 2, white)

	// Draw status indicator in bottom-right corner
	drawIndicator(img)

	f, _ := os.Create(filename)
	defer f.Close()
	png.Encode(f, img)
}

func drawRedX(img *image.NRGBA) {
	// Small red circle background at bottom-right
	cx, cy, r := 25.0, 25.0, 6.0
	red := color.NRGBA{220, 50, 50, 255}
	white := color.NRGBA{255, 255, 255, 255}

	for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
		for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
			if x < 0 || x >= traySize || y < 0 || y >= traySize {
				continue
			}
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist <= r {
				img.SetNRGBA(x, y, red)
			}
		}
	}

	// Draw X
	drawLine(img, 22, 22, 28, 28, 1, white)
	drawLine(img, 28, 22, 22, 28, 1, white)
}

func drawGreenCheck(img *image.NRGBA) {
	// Small green circle background at bottom-right
	cx, cy, r := 25.0, 25.0, 6.0
	green := color.NRGBA{50, 180, 50, 255}
	white := color.NRGBA{255, 255, 255, 255}

	for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
		for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
			if x < 0 || x >= traySize || y < 0 || y >= traySize {
				continue
			}
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist <= r {
				img.SetNRGBA(x, y, green)
			}
		}
	}

	// Draw checkmark ✓
	drawLine(img, 22, 25, 24, 28, 1, white)
	drawLine(img, 24, 28, 29, 22, 1, white)
}

func drawLine(img *image.NRGBA, x1, y1, x2, y2, thickness int, c color.NRGBA) {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return
	}
	steps := int(length * 3)
	halfT := float64(thickness) / 2

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		px := float64(x1) + dx*t
		py := float64(y1) + dy*t

		for iy := int(py - halfT); iy <= int(py+halfT); iy++ {
			for ix := int(px - halfT); ix <= int(px+halfT); ix++ {
				if ix >= 0 && ix < traySize && iy >= 0 && iy < traySize {
					ddx := float64(ix) + 0.5 - px
					ddy := float64(iy) + 0.5 - py
					if ddx*ddx+ddy*ddy <= (halfT+0.5)*(halfT+0.5) {
						img.SetNRGBA(ix, iy, c)
					}
				}
			}
		}
	}
}
