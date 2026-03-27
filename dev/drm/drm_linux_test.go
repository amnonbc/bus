package drm

import (
	"image"
	"image/color"
	"testing"
)

func newTestDevice(width, height int) *Device {
	stride := width * 4 // ABGR8888 is always 4 bytes per pixel
	return &Device{
		width:  width,
		height: height,
		stride: stride,
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

// TestBlit verifies that Device.Blit writes raw RGBA pixels unchanged
// regardless of what rotate was passed to Open. Rotation is handled in
// hardware via the DRM plane rotation property, so Blit always copies
// pixels straight through.
func TestBlit(t *testing.T) {
	img := newTestImage(16, 16)
	d := newTestDevice(16, 16)
	d.Blit(img)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			dstOff := y*d.stride + x*4
			srcOff := y*img.Stride + x*4
			if d.data[dstOff] != img.Pix[srcOff] ||
				d.data[dstOff+1] != img.Pix[srcOff+1] ||
				d.data[dstOff+2] != img.Pix[srcOff+2] ||
				d.data[dstOff+3] != img.Pix[srcOff+3] {
				t.Errorf("pixel (%d,%d): got %02x%02x%02x%02x want %02x%02x%02x%02x",
					x, y,
					d.data[dstOff], d.data[dstOff+1], d.data[dstOff+2], d.data[dstOff+3],
					img.Pix[srcOff], img.Pix[srcOff+1], img.Pix[srcOff+2], img.Pix[srcOff+3])
			}
		}
	}
}

func BenchmarkBlit(b *testing.B) {
	d := newTestDevice(800, 480)
	img := newTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Blit(img)
	}
}
