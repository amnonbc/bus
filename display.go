package main

import (
	"image"
	"log/slog"
	"os"
	"os/exec"
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
		slog.Info("hardware display unavailable", "err", err)
		hw = &fallbackDisplay{}
	} else {
		defer hw.Close()
	}

	buf, err := newFrameBuffer(hw.Width(), hw.Height(), hw, invert)
	if err != nil {
		return err
	}
	defer buf.close()
	newHTTPPreview(buf, flip).register()
	slog.Info("preview server", "url", "http://localhost:8080")
	go listenHTTP()

	// Try to open browser on non-Linux platforms
	if _, ok := hw.(*fallbackDisplay); ok {
		err := exec.Command("open", "http://localhost:8080").Start()
		if err != nil {
			slog.Debug("could not open browser", "err", err)
		}
	}

	buf.runLoop(active, weather, notify)
	return nil
}

// fallbackDisplay is used on non-Linux platforms or when pidisp.Open fails.
type fallbackDisplay struct{}

const (
	fallbackWidth  = 800
	fallbackHeight = 480
)

func (fallbackDisplay) Width() int              { return fallbackWidth }
func (fallbackDisplay) Height() int             { return fallbackHeight }
func (fallbackDisplay) Blit(*image.RGBA)        {}
func (fallbackDisplay) Close()                  {}