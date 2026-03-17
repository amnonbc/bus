//go:build !linux

// display_other.go runs the HTTP preview server on non-Linux platforms,
// rendering a fresh frame every second into the shared httpPreview.
package main

import (
	"log/slog"
	"sync/atomic"
	"time"
)

// displayWidth and displayHeight match the real Pi screen dimensions.
const (
	displayWidth  = 800
	displayHeight = 480
)

func runDisplay(active *atomic.Pointer[timeTable], weather *atomic.Pointer[string], _ bool, notify <-chan struct{}) error {
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

	p := newHTTPPreview(displayWidth, displayHeight)
	p.register()
	slog.Info("preview server", "url", "http://localhost:8080")
	go listenHTTP()

	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
		case <-notify:
		}
		renderFrame(p.backBuf(), bigFace, smallFace, active.Load(), *weather.Load())
		p.publishFrame()
	}
	return nil
}
