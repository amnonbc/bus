package fb

import (
	"image"
	"image/color"
	"testing"
)

func newTestDevice(width, height, bpp int, rotate bool) *Device {
	bytesPerPixel := bpp / 8
	stride := width * bytesPerPixel
	vinfo := fbVarScreenInfo{
		XRes:         uint32(width),
		YRes:         uint32(height),
		BitsPerPixel: uint32(bpp),
	}
	// RGB565: R at bit 11 (5 bits), G at bit 5 (6 bits), B at bit 0 (5 bits).
	// RGB8888: R at bit 0, G at bit 8, B at bit 16.
	if bpp == 16 {
		vinfo.Red = fbBitField{Offset: 11}
		vinfo.Green = fbBitField{Offset: 5}
		vinfo.Blue = fbBitField{Offset: 0}
	} else {
		vinfo.Red = fbBitField{Offset: 0}
		vinfo.Green = fbBitField{Offset: 8}
		vinfo.Blue = fbBitField{Offset: 16}
		vinfo.Transp = fbBitField{Offset: 24}
	}
	return &Device{
		width:  width,
		height: height,
		stride: stride,
		bpp:    bpp,
		rotate: rotate,
		vinfo:  vinfo,
		data:   make([]byte, stride*height),
	}
}

func newTestImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Fill with a non-trivial pattern so the compiler can't optimise it away.
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(x),
				G: uint8(y),
				B: uint8(x + y),
				A: 0xff,
			})
		}
	}
	return img
}

func BenchmarkBlit16(b *testing.B) {
	d := newTestDevice(800, 480, 16, false)
	img := newTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Blit(img)
	}
}

func BenchmarkBlit16Rotate(b *testing.B) {
	d := newTestDevice(800, 480, 16, true)
	img := newTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Blit(img)
	}
}

func BenchmarkBlit32(b *testing.B) {
	d := newTestDevice(800, 480, 32, false)
	img := newTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Blit(img)
	}
}

func BenchmarkBlit32Rotate(b *testing.B) {
	d := newTestDevice(800, 480, 32, true)
	img := newTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Blit(img)
	}
}
