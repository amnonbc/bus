// drm_linux.go drives the display via DRM/KMS, writing XRGB8888 dumb buffers.
// This is preferred over the fbdev path because the pixel-format conversion
// (RGBA → XRGB8888) is a simple R/B channel swap rather than a full RGB565
// dither, and the display controller handles the rest.
package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"log/slog"
	"os"
	"syscall"
	"unsafe"
)

// drmIoWR builds a DRM ioctl request number for a read+write ioctl.
// Encoding: bits 31-30 = direction (3=R+W), bits 29-16 = struct size,
// bits 15-8 = type ('d'=0x64 for DRM), bits 7-0 = command number.
func drmIoWR(nr, size uintptr) uintptr {
	return (3 << 30) | (size << 16) | (0x64 << 8) | nr
}

// The following structs mirror the Linux kernel UAPI headers in <drm/drm_mode.h>.
// All fields use fixed-width types so the layout matches on 32- and 64-bit ARM.

// drmModeRes mirrors struct drm_mode_card_res.
type drmModeRes struct {
	FbIDPtr         uint64
	CrtcIDPtr       uint64
	ConnectorIDPtr  uint64
	EncoderIDPtr    uint64
	CountFbs        uint32
	CountCrtcs      uint32
	CountConnectors uint32
	CountEncoders   uint32
	MinWidth        uint32
	MaxWidth        uint32
	MinHeight       uint32
	MaxHeight       uint32
}

// drmModeGetConnector mirrors struct drm_mode_get_connector.
type drmModeGetConnector struct {
	EncodersPtr     uint64
	ModesPtr        uint64
	PropsPtr        uint64
	PropValuesPtr   uint64
	CountModes      uint32
	CountProps      uint32
	CountEncoders   uint32
	EncoderID       uint32
	ConnectorID     uint32
	ConnectorType   uint32
	ConnectorTypeID uint32
	Connection      uint32
	MmWidth         uint32
	MmHeight        uint32
	Subpixel        uint32
	Pad             uint32
}

// drmModeModeInfo mirrors struct drm_mode_modeinfo.
type drmModeModeInfo struct {
	Clock      uint32
	Hdisplay   uint16
	HsyncStart uint16
	HsyncEnd   uint16
	Htotal     uint16
	Hskew      uint16
	Vdisplay   uint16
	VsyncStart uint16
	VsyncEnd   uint16
	Vtotal     uint16
	Vscan      uint16
	VRefresh   uint32
	Flags      uint32
	Type       uint32
	Name       [32]byte
}

// drmModeGetEncoder mirrors struct drm_mode_get_encoder.
type drmModeGetEncoder struct {
	EncoderID      uint32
	EncoderType    uint32
	CrtcID         uint32
	PossibleCrtcs  uint32
	PossibleClones uint32
}

// drmModeCreateDumb mirrors struct drm_mode_create_dumb.
type drmModeCreateDumb struct {
	Height uint32
	Width  uint32
	Bpp    uint32
	Flags  uint32
	Handle uint32
	Pitch  uint32
	Size   uint64
}

// drmModeMapDumb mirrors struct drm_mode_map_dumb.
type drmModeMapDumb struct {
	Handle uint32
	Pad    uint32
	Offset uint64
}

// drmModeFBCmd mirrors struct drm_mode_fb_cmd (legacy ADDFB).
type drmModeFBCmd struct {
	FbID   uint32
	Width  uint32
	Height uint32
	Pitch  uint32
	Bpp    uint32
	Depth  uint32
	Handle uint32
}

// drmModeCrtc mirrors struct drm_mode_crtc.
type drmModeCrtc struct {
	SetConnectorsPtr uint64
	CountConnectors  uint32
	CrtcID           uint32
	FbID             uint32
	X                uint32
	Y                uint32
	GammaSize        uint32
	ModeValid        uint32
	Mode             drmModeModeInfo
}

// drmModeDestroyDumb mirrors struct drm_mode_destroy_dumb.
type drmModeDestroyDumb struct {
	Handle uint32
}

// ioctl request numbers, computed from struct sizes at init time.
var (
	ioctlModeGetResources = drmIoWR(0xA0, unsafe.Sizeof(drmModeRes{}))
	ioctlModeGetConnector = drmIoWR(0xA7, unsafe.Sizeof(drmModeGetConnector{}))
	ioctlModeGetEncoder   = drmIoWR(0xA6, unsafe.Sizeof(drmModeGetEncoder{}))
	ioctlModeCreateDumb   = drmIoWR(0xB2, unsafe.Sizeof(drmModeCreateDumb{}))
	ioctlModeMapDumb      = drmIoWR(0xB3, unsafe.Sizeof(drmModeMapDumb{}))
	ioctlModeAddFB        = drmIoWR(0xAE, unsafe.Sizeof(drmModeFBCmd{}))
	ioctlModeSetCRTC      = drmIoWR(0xA2, unsafe.Sizeof(drmModeCrtc{}))
	ioctlModeDestroyDumb  = drmIoWR(0xB4, unsafe.Sizeof(drmModeDestroyDumb{}))
)

func drmIoctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

type drmDevice struct {
	file   *os.File
	fd     uintptr
	width  int
	height int
	stride int
	handle uint32
	data   []byte
}

// openDRM opens the DRM device, finds the first connected connector, creates a
// 32 bpp dumb buffer, registers it as a framebuffer and sets the CRTC.
func openDRM(dev string) (*drmDevice, error) {
	f, err := os.OpenFile(dev, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dev, err)
	}
	fd := f.Fd()

	// First call: discover how many connectors exist.
	var res drmModeRes
	err = drmIoctl(fd, ioctlModeGetResources, unsafe.Pointer(&res))
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("DRM_IOCTL_MODE_GETRESOURCES: %w", err)
	}
	if res.CountConnectors == 0 {
		f.Close()
		return nil, fmt.Errorf("no DRM connectors found")
	}

	// Second call: fetch only the connector ID list. Zero out the counts and
	// pointers for fbs, crtcs, and encoders so the kernel doesn't try to write
	// them to address 0 (which would return EFAULT).
	connectorIDs := make([]uint32, res.CountConnectors)
	res.ConnectorIDPtr = uint64(uintptr(unsafe.Pointer(&connectorIDs[0])))
	res.CountFbs = 0
	res.FbIDPtr = 0
	res.CountCrtcs = 0
	res.CrtcIDPtr = 0
	res.CountEncoders = 0
	res.EncoderIDPtr = 0
	err = drmIoctl(fd, ioctlModeGetResources, unsafe.Pointer(&res))
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("DRM_IOCTL_MODE_GETRESOURCES (ids): %w", err)
	}

	// Find the first connector that has at least one mode.
	// We check CountModes rather than the Connection field because some drivers
	// (including VC4 on the Pi) report DRM_MODE_UNKNOWNCONNECTION even when a
	// display is physically attached; CountModes > 0 is the reliable indicator.
	var connID uint32
	var encoderID uint32
	var mode drmModeModeInfo
	found := false
	for _, id := range connectorIDs {
		conn := drmModeGetConnector{ConnectorID: id}
		err = drmIoctl(fd, ioctlModeGetConnector, unsafe.Pointer(&conn))
		if err != nil || conn.CountModes == 0 {
			continue
		}
		// Fetch only the mode list. Zero out props and encoders so the kernel
		// doesn't attempt to write them to address 0 (EFAULT).
		modes := make([]drmModeModeInfo, conn.CountModes)
		conn.ModesPtr = uint64(uintptr(unsafe.Pointer(&modes[0])))
		conn.CountProps = 0
		conn.PropsPtr = 0
		conn.PropValuesPtr = 0
		conn.CountEncoders = 0
		conn.EncodersPtr = 0
		err = drmIoctl(fd, ioctlModeGetConnector, unsafe.Pointer(&conn))
		if err != nil {
			continue
		}
		connID = id
		encoderID = conn.EncoderID
		mode = modes[0]
		found = true
		break
	}
	if !found {
		f.Close()
		return nil, fmt.Errorf("no DRM connector with modes found")
	}

	// Get the CRTC currently bound to this connector's encoder.
	var enc drmModeGetEncoder
	enc.EncoderID = encoderID
	err = drmIoctl(fd, ioctlModeGetEncoder, unsafe.Pointer(&enc))
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("DRM_IOCTL_MODE_GETENCODER: %w", err)
	}
	crtcID := enc.CrtcID

	width := int(mode.Hdisplay)
	height := int(mode.Vdisplay)

	// Create a 32 bpp dumb buffer. The kernel fills in Handle, Pitch, and Size.
	var dumb drmModeCreateDumb
	dumb.Width = uint32(width)
	dumb.Height = uint32(height)
	dumb.Bpp = 32
	err = drmIoctl(fd, ioctlModeCreateDumb, unsafe.Pointer(&dumb))
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("DRM_IOCTL_MODE_CREATE_DUMB: %w", err)
	}

	// Register it as a DRM framebuffer (legacy ADDFB; bpp=32 depth=24 → XRGB8888).
	var fb drmModeFBCmd
	fb.Width = uint32(width)
	fb.Height = uint32(height)
	fb.Pitch = dumb.Pitch
	fb.Bpp = 32
	fb.Depth = 24
	fb.Handle = dumb.Handle
	err = drmIoctl(fd, ioctlModeAddFB, unsafe.Pointer(&fb))
	if err != nil {
		destroy := drmModeDestroyDumb{Handle: dumb.Handle}
		drmIoctl(fd, ioctlModeDestroyDumb, unsafe.Pointer(&destroy))
		f.Close()
		return nil, fmt.Errorf("DRM_IOCTL_MODE_ADDFB: %w", err)
	}

	// Get the mmap offset for the dumb buffer.
	var mapDumb drmModeMapDumb
	mapDumb.Handle = dumb.Handle
	err = drmIoctl(fd, ioctlModeMapDumb, unsafe.Pointer(&mapDumb))
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("DRM_IOCTL_MODE_MAP_DUMB: %w", err)
	}

	data, err := syscall.Mmap(
		int(fd), int64(mapDumb.Offset), int(dumb.Size),
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED,
	)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("mmap dumb buffer: %w", err)
	}

	// Point the CRTC at our new framebuffer.
	crtc := drmModeCrtc{
		CrtcID:           crtcID,
		FbID:             fb.FbID,
		ModeValid:        1,
		Mode:             mode,
		SetConnectorsPtr: uint64(uintptr(unsafe.Pointer(&connID))),
		CountConnectors:  1,
	}
	err = drmIoctl(fd, ioctlModeSetCRTC, unsafe.Pointer(&crtc))
	if err != nil {
		syscall.Munmap(data)
		f.Close()
		return nil, fmt.Errorf("DRM_IOCTL_MODE_SETCRTC: %w", err)
	}

	slog.Info("DRM display", "width", width, "height", height,
		"stride", dumb.Pitch, "crtc", crtcID)

	return &drmDevice{
		file:   f,
		fd:     fd,
		width:  width,
		height: height,
		stride: int(dumb.Pitch),
		handle: dumb.Handle,
		data:   data,
	}, nil
}

func (d *drmDevice) close() {
	syscall.Munmap(d.data)
	var destroy drmModeDestroyDumb
	destroy.Handle = d.handle
	drmIoctl(d.fd, ioctlModeDestroyDumb, unsafe.Pointer(&destroy))
	d.file.Close()
}

// blit copies img to the DRM dumb buffer in XRGB8888 format.
//
// XRGB8888 in little-endian memory: byte[0]=B, byte[1]=G, byte[2]=R, byte[3]=X.
// image.RGBA.Pix layout:            byte[0]=R, byte[1]=G, byte[2]=B, byte[3]=A.
// Each pixel is read as a uint32, R and B are swapped, and the result is written
// back as a uint32 — one load and one store per pixel, no dithering required.
func (d *drmDevice) blit(img *image.RGBA, rotate bool) {
	for y := 0; y < d.height; y++ {
		srcRow := img.Pix[y*img.Stride:]

		dstY := y
		if rotate {
			dstY = d.height - 1 - y
		}

		srcOff := 0
		dstOff := dstY * d.stride
		dstStep := 4
		if rotate {
			dstOff += (d.width - 1) * 4
			dstStep = -4
		}

		for x := 0; x < d.width; x++ {
			// Read source as uint32 (LE): R | G<<8 | B<<16 | A<<24.
			// Rearrange to XRGB8888: B | G<<8 | R<<16 | 0<<24.
			// G stays in byte 1; R (byte 0) moves to byte 2; B (byte 2) moves to byte 0.
			src := binary.LittleEndian.Uint32(srcRow[srcOff:])
			px := (src & 0x0000FF00) | (src&0x000000FF)<<16 | (src>>16)&0xFF
			binary.LittleEndian.PutUint32(d.data[dstOff:], px)
			srcOff += 4
			dstOff += dstStep
		}
	}
}
