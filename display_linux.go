// display_linux.go drives the Linux display, blitting rendered frames via
// DRM/KMS if available, falling back to /dev/fb0.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"

	"github.com/amnonbc/pidisp/drm"
	"github.com/amnonbc/pidisp/fb"
)

func logBoardModel() {
	data, err := os.ReadFile("/proc/device-tree/model")
	if err != nil {
		return
	}
	slog.Info("board", "model", strings.TrimRight(string(data), "\x00"))
}

func runDisplay(active *atomic.Pointer[timeTable], weather *atomic.Pointer[string], rotate bool, debug bool, forceFB bool, invert bool, notify <-chan struct{}, flip func()) error {
	logBoardModel()

	var hw blitter

	if debug {
		for _, card := range []string{"/dev/dri/card0", "/dev/dri/card1", "/dev/dri/card2"} {
			drm.LogPlaneFormats(card)
		}
	}

	if !forceFB {
		d, err := drm.Open("/dev/dri/card0", rotate)
		if err == nil {
			defer d.Close()
			hw = d
		} else {
			slog.Info("DRM unavailable, falling back to framebuffer", "err", err)
		}
	}

	if hw == nil {
		f, err := fb.Open("/dev/fb0", rotate)
		if err != nil {
			return fmt.Errorf("framebuffer: %w", err)
		}
		defer f.Close()
		hw = f
	}

	buf, err := newFrameBuffer(hw.Width(), hw.Height(), hw, invert)
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
