// display_linux.go drives the Linux display, blitting rendered frames via
// DRM/KMS if available, falling back to /dev/fb0.
package main

import (
	"bus/dev/drm"
	"bus/dev/fb"
	"fmt"
	"log/slog"
	"sync/atomic"
)

func runDisplay(active *atomic.Pointer[timeTable], weather *atomic.Pointer[string], rotate bool, notify <-chan struct{}, flip func()) error {
	var hw blitter

	d, err := drm.Open("/dev/dri/card0", rotate)
	if err == nil {
		defer d.Close()
		hw = d
	} else {
		slog.Info("DRM unavailable, falling back to framebuffer", "err", err)
		f, err := fb.Open("/dev/fb0", rotate)
		if err != nil {
			return fmt.Errorf("framebuffer: %w", err)
		}
		defer f.Close()
		hw = f
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

	buf := newFrameBuffer(hw.Width(), hw.Height(), bigFace, smallFace, hw)
	newHTTPPreview(buf, flip).register()
	slog.Info("preview server", "url", "http://localhost:8080")
	go listenHTTP()
	buf.runLoop(active, weather, notify)
	return nil
}
