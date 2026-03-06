//go:build ignore

// Generates a Windows .ico file from appicon.png (256x256 PNG wrapped in ICO container).
package main

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"os"
)

func main() {
	pngData, err := os.ReadFile("appicon.png")
	if err != nil {
		panic(err)
	}

	// Decode to get dimensions
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		panic(err)
	}
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// ICO format: header + 1 entry + PNG data
	var buf bytes.Buffer

	// ICONDIR header (6 bytes)
	binary.Write(&buf, binary.LittleEndian, uint16(0))    // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))    // type: 1 = ICO
	binary.Write(&buf, binary.LittleEndian, uint16(1))    // count: 1 image

	// ICONDIRENTRY (16 bytes)
	wByte := uint8(0) // 0 means 256
	hByte := uint8(0)
	if w < 256 {
		wByte = uint8(w)
	}
	if h < 256 {
		hByte = uint8(h)
	}
	buf.WriteByte(wByte)                                          // width
	buf.WriteByte(hByte)                                          // height
	buf.WriteByte(0)                                              // color palette
	buf.WriteByte(0)                                              // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))            // color planes
	binary.Write(&buf, binary.LittleEndian, uint16(32))           // bits per pixel
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngData))) // size of PNG data
	binary.Write(&buf, binary.LittleEndian, uint32(22))           // offset (6 + 16 = 22)

	// PNG data
	buf.Write(pngData)

	os.WriteFile("appicon.ico", buf.Bytes(), 0644)
}
