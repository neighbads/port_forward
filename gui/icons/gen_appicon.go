//go:build ignore

// Generates app icon: two arrows (port forwarding) on a blue rounded background.
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const size = 256

func main() {
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Draw rounded blue background
	center := float64(size) / 2
	bgRadius := float64(size)/2 - 8
	bgColor := color.RGBA{50, 120, 200, 255}
	bgDark := color.RGBA{35, 90, 160, 255}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - center
			dy := float64(y) - center
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist <= bgRadius {
				// Simple gradient: darker at bottom
				t := float64(y) / float64(size)
				r := uint8(float64(bgColor.R)*(1-t*0.3) + float64(bgDark.R)*t*0.3)
				g := uint8(float64(bgColor.G)*(1-t*0.3) + float64(bgDark.G)*t*0.3)
				b := uint8(float64(bgColor.B)*(1-t*0.3) + float64(bgDark.B)*t*0.3)
				if dist > bgRadius-2 {
					// Anti-alias edge
					alpha := uint8(255 * (bgRadius - dist) / 2)
					img.Set(x, y, color.RGBA{r, g, b, alpha})
				} else {
					img.Set(x, y, color.RGBA{r, g, b, 255})
				}
			}
		}
	}

	white := color.RGBA{255, 255, 255, 255}

	// Draw right arrow (top): represents outgoing traffic →
	// Arrow body: horizontal line
	drawThickLine(img, 70, 90, 186, 90, 10, white)
	// Arrow head
	drawThickLine(img, 166, 70, 190, 90, 8, white)
	drawThickLine(img, 166, 110, 190, 90, 8, white)

	// Draw left arrow (bottom): represents incoming traffic ←
	drawThickLine(img, 70, 166, 186, 166, 10, white)
	// Arrow head
	drawThickLine(img, 70, 166, 94, 146, 8, white)
	drawThickLine(img, 70, 166, 94, 186, 8, white)

	f, _ := os.Create("appicon.png")
	defer f.Close()
	png.Encode(f, img)
}

func drawThickLine(img *image.RGBA, x1, y1, x2, y2, thickness int, c color.RGBA) {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return
	}

	steps := int(length * 2)
	halfT := float64(thickness) / 2

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		cx := float64(x1) + dx*t
		cy := float64(y1) + dy*t

		for py := int(cy - halfT); py <= int(cy+halfT); py++ {
			for px := int(cx - halfT); px <= int(cx+halfT); px++ {
				ddx := float64(px) - cx
				ddy := float64(py) - cy
				if ddx*ddx+ddy*ddy <= halfT*halfT {
					if px >= 0 && px < size && py >= 0 && py < size {
						img.Set(px, py, c)
					}
				}
			}
		}
	}
}
