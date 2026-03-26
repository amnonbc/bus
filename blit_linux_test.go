package main

import (
	"image"
	"image/color"
	"testing"
)

func newTestFB(width, height, bpp int) *fbDevice {
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
	return &fbDevice{
		width:  width,
		height: height,
		stride: stride,
		bpp:    bpp,
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
	fb := newTestFB(800, 480, 16)
	img := newTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fb.blit(img, false)
	}
}

func BenchmarkBlit16Rotate(b *testing.B) {
	fb := newTestFB(800, 480, 16)
	img := newTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fb.blit(img, true)
	}
}

func BenchmarkBlit32(b *testing.B) {
	fb := newTestFB(800, 480, 32)
	img := newTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fb.blit(img, false)
	}
}

func BenchmarkBlit32Rotate(b *testing.B) {
	fb := newTestFB(800, 480, 32)
	img := newTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fb.blit(img, true)
	}
}

func newTestDRM(width, height int) *drmDevice {
	stride := width * 4 // ABGR8888 is always 4 bytes per pixel
	return &drmDevice{
		width:  width,
		height: height,
		stride: stride,
		data:   make([]byte, stride*height),
	}
}

// TestDRMBlit verifies that drmDevice.blit writes raw RGBA pixels unchanged
// regardless of the rotate flag. Rotation is handled in hardware via the DRM
// plane rotation property, so blit always copies pixels straight through.
func TestDRMBlit(t *testing.T) {
	for _, rotate := range []bool{false, true} {
		img := newTestImage(16, 16)
		drm := newTestDRM(16, 16)
		drm.blit(img, rotate)
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				dstOff := y*drm.stride + x*4
				srcOff := y*img.Stride + x*4
				if drm.data[dstOff] != img.Pix[srcOff] ||
					drm.data[dstOff+1] != img.Pix[srcOff+1] ||
					drm.data[dstOff+2] != img.Pix[srcOff+2] ||
					drm.data[dstOff+3] != img.Pix[srcOff+3] {
					t.Errorf("rotate=%v pixel (%d,%d): got %02x%02x%02x%02x want %02x%02x%02x%02x",
						rotate, x, y,
						drm.data[dstOff], drm.data[dstOff+1], drm.data[dstOff+2], drm.data[dstOff+3],
						img.Pix[srcOff], img.Pix[srcOff+1], img.Pix[srcOff+2], img.Pix[srcOff+3])
				}
			}
		}
	}
}

func BenchmarkBlitDRM(b *testing.B) {
	d := newTestDRM(800, 480)
	img := newTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.blit(img, false)
	}
}

func BenchmarkBlitDRMRotate(b *testing.B) {
	d := newTestDRM(800, 480)
	img := newTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.blit(img, true)
	}
}