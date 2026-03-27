//go:build !linux

// display_other.go runs the HTTP preview server on non-Linux platforms.
package main

import (
	"log/slog"
	"sync/atomic"
)

// displayWidth and displayHeight match the real Pi screen dimensions.
const (
	displayWidth  = 800
	displayHeight = 480
)

func runDisplay(active *atomic.Pointer[timeTable], weather *atomic.Pointer[string], _ bool, notify <-chan struct{}, flip func()) error {
	buf, err := newFrameBuffer(displayWidth, displayHeight, noopBlitter{})
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
