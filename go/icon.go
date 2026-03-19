//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

// generateIcon returns a 32×32 Spotify-green circle wrapped in ICO format.
// fyne.io/systray on Windows writes the bytes to a temp file and loads it via
// LoadImageW(IMAGE_ICON | LR_LOADFROMFILE), which expects ICO binary — not PNG.
func generateIcon() []byte {
	const size = 32

	// Draw Spotify-green (#1DB954) circle
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx, cy, r := 16.0, 16.0, 15.0
	green := color.NRGBA{R: 29, G: 185, B: 84, A: 255}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, green)
			}
		}
	}

	// Encode as PNG
	var pngBuf bytes.Buffer
	png.Encode(&pngBuf, img)
	pngData := pngBuf.Bytes()

	// Wrap in ICO container (Vista+ supports PNG-in-ICO)
	// Layout: ICONDIR(6) + ICONDIRENTRY(16) + PNG data
	var ico bytes.Buffer
	// ICONDIR
	binary.Write(&ico, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&ico, binary.LittleEndian, uint16(1)) // type = icon
	binary.Write(&ico, binary.LittleEndian, uint16(1)) // image count
	// ICONDIRENTRY
	ico.WriteByte(byte(size))                                              // width
	ico.WriteByte(byte(size))                                              // height
	ico.WriteByte(0)                                                       // color count (0 = no palette)
	ico.WriteByte(0)                                                       // reserved
	binary.Write(&ico, binary.LittleEndian, uint16(1))                    // color planes
	binary.Write(&ico, binary.LittleEndian, uint16(32))                   // bits per pixel
	binary.Write(&ico, binary.LittleEndian, uint32(len(pngData)))         // image size
	binary.Write(&ico, binary.LittleEndian, uint32(6+16))                 // offset (after headers)
	ico.Write(pngData)
	return ico.Bytes()
}
