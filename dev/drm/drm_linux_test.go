package drm

import (
	"bus/dev/fb"
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

// TestBlit verifies that Device.Blit writes raw RGBA pixels unchanged
// regardless of what rotate was passed to Open. Rotation is handled in
// hardware via the DRM plane rotation property, so Blit always copies
// pixels straight through.
func TestBlit(t *testing.T) {
	img := fb.NewTestImage(16, 16)
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

// TestBlitMatchesFB32 verifies that drm.Device.Blit produces the same bytes as
// fb.Device.Blit for a 32 bpp framebuffer. Both use RGBA byte order (R at
// byte[0], G at byte[1], B at byte[2], A at byte[3]), so their outputs must
// be identical pixel-for-pixel.
func TestBlitMatchesFB32(t *testing.T) {
	img := fb.NewTestImage(16, 16)
	drm := newTestDevice(16, 16)
	fbDev := fb.NewTestDevice(16, 16, 32, false)
	drm.Blit(img)
	fbDev.Blit(img)
	drmData := drm.data
	fbData := fbDev.Data()
	for i, b := range drmData {
		if b != fbData[i] {
			x := (i % drm.stride) / 4
			y := i / drm.stride
			t.Errorf("pixel (%d,%d) byte %d: drm=%02x fb=%02x", x, y, i%4, b, fbData[i])
		}
	}
}

func BenchmarkBlit(b *testing.B) {
	d := newTestDevice(800, 480)
	img := fb.NewTestImage(800, 480)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Blit(img)
	}
}
