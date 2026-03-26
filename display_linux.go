// display_linux.go drives the Linux framebuffer, blitting rendered frames to /dev/fb0.
package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const fbioGetVScreenInfo = 0x4600

// bayer4x4 is a normalised 4×4 Bayer ordered-dither threshold matrix scaled
// to the range [0, 15]. Adding this value to an 8-bit channel before
// right-shifting spreads quantisation error across a 4×4 pixel tile.
var bayer4x4 = [4][4]uint8{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

// clamp8 clamps v to [0, 255].
func clamp8(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

type fbBitField struct {
	Offset   uint32
	Length   uint32
	MsbRight uint32
}

// fbVarScreenInfo mirrors struct fb_var_screeninfo from <linux/fb.h>.
// All fields are fixed-width types so the layout is the same on 32- and
// 64-bit ARM Linux.
type fbVarScreenInfo struct {
	XRes         uint32
	YRes         uint32
	XResVirtual  uint32
	YResVirtual  uint32
	XOffset      uint32
	YOffset      uint32
	BitsPerPixel uint32
	Grayscale    uint32
	Red          fbBitField
	Green        fbBitField
	Blue         fbBitField
	Transp       fbBitField
	NonStd       uint32
	Activate     uint32
	Height       uint32
	Width        uint32
	AccelFlags   uint32
	PixClock     uint32
	LeftMargin   uint32
	RightMargin  uint32
	UpperMargin  uint32
	LowerMargin  uint32
	HSyncLen     uint32
	VSyncLen     uint32
	Sync         uint32
	VMode        uint32
	Rotate       uint32
	Colorspace   uint32
	Reserved     [4]uint32
}

type fbDevice struct {
	width  int
	height int
	stride int
	bpp    int
	vinfo  fbVarScreenInfo
	data   []byte
}

func openFB(dev string) (*fbDevice, error) {
	f, err := os.OpenFile(dev, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dev, err)
	}

	var vinfo fbVarScreenInfo
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		fbioGetVScreenInfo,
		uintptr(unsafe.Pointer(&vinfo)),
	)
	if errno != 0 {
		f.Close()
		return nil, fmt.Errorf("ioctl FBIOGET_VSCREENINFO: %w", errno)
	}

	width := int(vinfo.XRes)
	height := int(vinfo.YRes)
	bpp := int(vinfo.BitsPerPixel)

	stride := width * bpp / 8
	if raw, err := os.ReadFile("/sys/class/graphics/fb0/stride"); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil && n > 0 {
			stride = n
		}
	}

	fbSize := stride * height
	data, err := syscall.Mmap(
		int(f.Fd()), 0, fbSize,
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED,
	)
	f.Close()
	if err != nil {
		return nil, fmt.Errorf("mmap: %w", err)
	}

	slog.Info("framebuffer",
		"width", width, "height", height,
		"bpp", bpp, "stride", stride,
		"red", vinfo.Red.Offset, "green", vinfo.Green.Offset, "blue", vinfo.Blue.Offset)

	return &fbDevice{
		width:  width,
		height: height,
		stride: stride,
		bpp:    bpp,
		vinfo:  vinfo,
		data:   data,
	}, nil
}

func (fb *fbDevice) close() {
	syscall.Munmap(fb.data)
}

// blit copies img to the framebuffer, optionally rotating 180 degrees, and
// converting from RGBA to the hardware pixel format described by the vinfo bitfields.
func (fb *fbDevice) blit(img *image.RGBA, rotate bool) {
	rOff := fb.vinfo.Red.Offset
	gOff := fb.vinfo.Green.Offset
	bOff := fb.vinfo.Blue.Offset
	aOff := fb.vinfo.Transp.Offset
	bytesPerPixel := fb.bpp / 8

	for y := 0; y < fb.height; y++ {
		// Each row in the source image is a slice of 4-byte RGBA pixels.
		srcRow := img.Pix[y*img.Stride:]

		// When rotating 180°, row 0 maps to the last row of the framebuffer,
		// row 1 to the second-to-last, and so on.
		dstY := y
		if rotate {
			dstY = fb.height - 1 - y
		}

		// Walk src forward by 4 bytes per pixel; walk dst forward or backward
		// by bytesPerPixel, avoiding a per-pixel multiply.
		srcOff := 0
		dstOff := dstY * fb.stride
		dstStep := bytesPerPixel
		if rotate {
			dstOff += (fb.width - 1) * bytesPerPixel
			dstStep = -bytesPerPixel
		}

		// Cache the bayer row for this y — it is constant across the x loop.
		bayerRow := bayer4x4[y&3]

		// Pack the pixel into the hardware format. The vinfo bitfields
		// tell us at which bit offset each colour channel sits, which
		// varies between framebuffer drivers and colour depths.
		switch fb.bpp {
		case 32:
			for x := 0; x < fb.width; x++ {
				r := srcRow[srcOff]
				g := srcRow[srcOff+1]
				b := srcRow[srcOff+2]
				a := srcRow[srcOff+3]
				// Shift each 8-bit channel to its hardware bit position and OR together.
				px := uint32(r)<<rOff | uint32(g)<<gOff |
					uint32(b)<<bOff | uint32(a)<<aOff
				binary.LittleEndian.PutUint32(fb.data[dstOff:], px)
				srcOff += 4
				dstOff += dstStep
			}
		case 16:
			for x := 0; x < fb.width; x++ {
				r := srcRow[srcOff]
				g := srcRow[srcOff+1]
				b := srcRow[srcOff+2]
				// RGB565 with Bayer 4×4 ordered dithering.
				// Without dithering, anti-aliased font edges quantise in
				// steps of 8 (R/B) or 4 (G), producing a visible staircase.
				// Adding a position-dependent threshold before the right-shift
				// spreads the quantisation error across neighbouring pixels.
				d := bayerRow[x&3]
				r5 := uint16(clamp8(int(r)+int(d>>3))) >> 3
				g6 := uint16(clamp8(int(g)+int(d>>2))) >> 2
				b5 := uint16(clamp8(int(b)+int(d>>3))) >> 3
				binary.LittleEndian.PutUint16(fb.data[dstOff:], r5<<uint(rOff)|g6<<uint(gOff)|b5<<uint(bOff))
				srcOff += 4
				dstOff += dstStep
			}
		}
	}
}

func runDisplay(active *atomic.Pointer[timeTable], weather *atomic.Pointer[string], rotate bool, notify <-chan struct{}, flip func()) error {
	var hw blitter
	var width, height int

	drm, err := openDRM("/dev/dri/card0", rotate)
	if err == nil {
		defer drm.close()
		hw = drm
		width = drm.width
		height = drm.height
	} else {
		slog.Info("DRM unavailable, falling back to framebuffer", "err", err)
		fb, err := openFB("/dev/fb0")
		if err != nil {
			return err
		}
		defer fb.close()
		if fb.bpp != 16 && fb.bpp != 32 {
			return fmt.Errorf("unsupported framebuffer depth: %d bpp", fb.bpp)
		}
		hw = fb
		width = fb.width
		height = fb.height
	}

	bigFace, err := newFace(100)
	if err != nil {
		return err
	}
	defer bigFace.Close()
	smallFace, err := newFace(32)
	if err != nil {
		return err
	}
	defer smallFace.Close()

	buf := newFrameBuffer(width, height)
	newHTTPPreview(buf, flip).register()
	slog.Info("preview server", "url", "http://localhost:8080")
	go listenHTTP()
	runLoop(buf, active, weather, bigFace, smallFace, hw, rotate, notify)
	return nil
}
