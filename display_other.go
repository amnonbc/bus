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

	buf := newFrameBuffer(displayWidth, displayHeight)
	newHTTPPreview(buf, flip).register()
	slog.Info("preview server", "url", "http://localhost:8080")
	go listenHTTP()
	runLoop(buf, active, weather, bigFace, smallFace, noopBlitter{}, false, notify)
	return nil
}
