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
	stride := width * 4 // XRGB8888 is always 4 bytes per pixel
	return &drmDevice{
		width:  width,
		height: height,
		stride: stride,
		data:   make([]byte, stride*height),
	}
}

// newTestFBXRGB returns an fbDevice configured for XRGB8888 pixel layout,
// matching the format written by drmDevice.blit.
func newTestFBXRGB(width, height int) *fbDevice {
	stride := width * 4
	return &fbDevice{
		width:  width,
		height: height,
		stride: stride,
		bpp:    32,
		vinfo: fbVarScreenInfo{
			XRes:         uint32(width),
			YRes:         uint32(height),
			BitsPerPixel: 32,
			Red:          fbBitField{Offset: 16},
			Green:        fbBitField{Offset: 8},
			Blue:         fbBitField{Offset: 0},
			Transp:       fbBitField{Offset: 24},
		},
		data: make([]byte, stride*height),
	}
}

func TestDRMMatchesFbdev(t *testing.T) {
	for _, rotate := range []bool{false, true} {
		img := newTestImage(16, 16)
		drm := newTestDRM(16, 16)
		fb := newTestFBXRGB(16, 16)
		drm.blit(img, rotate)
		fb.blit(img, rotate)
		// DRM writes X=0 in byte 3; fbdev writes A=0xFF. Only compare RGB bytes 0-2.
		for i := 0; i < len(drm.data); i += 4 {
			if drm.data[i] != fb.data[i] || drm.data[i+1] != fb.data[i+1] || drm.data[i+2] != fb.data[i+2] {
				t.Errorf("rotate=%v pixel %d: DRM %02x%02x%02x fb %02x%02x%02x",
					rotate, i/4,
					drm.data[i], drm.data[i+1], drm.data[i+2],
					fb.data[i], fb.data[i+1], fb.data[i+2])
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