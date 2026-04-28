// display_linux.go drives the Linux display, blitting rendered frames via
// DRM/KMS if available, falling back to /dev/fb0.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"

	"github.com/amnonbc/pidisp"
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

	hw, err := pidisp.Open(pidisp.Options{
		Rotate:  rotate,
		ForceFB: forceFB,
		Debug:   debug,
	})
	if err != nil {
		return fmt.Errorf("failed to open display: %w", err)
	}
	defer hw.Close()

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
