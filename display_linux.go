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

	buf, err := newFrameBuffer(hw.Width(), hw.Height(), hw)
	if err != nil {
		return err
	}
	defer buf.close()
	newHTTPPreview(buf, flip).register()
	slog.Info("preview server", "url", "http://localhost:8080")
	go listenHTTP()
	buf.runLoop(active, weather, notify)
	return nil
}
