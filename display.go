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

func runDisplay(active *atomic.Pointer[timeTable], weather *atomic.Pointer[string], opts fbOptions, notify <-chan struct{}, flip func()) error {
	logBoardModel()

	hw, err := pidisp.Open(pidisp.Options{
		Rotate:    opts.rotate,
		ForceFB:   opts.forceFB,
		Debug:     opts.debug,
		AsyncFlip: opts.async,
	})
	if err != nil {
		slog.Info("hardware display unavailable", "err", err)
		hw = newFallbackDisplay()
	} else {
		defer hw.Close()
	}

	buf, err := newFrameBuffer(hw.Width(), hw.Height(), hw, opts)
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
type fallbackDisplay struct {
	bufs   [2]*image.RGBA
	active int
}

const (
	fallbackWidth  = 800
	fallbackHeight = 480
)

func newFallbackDisplay() *fallbackDisplay {
	rect := image.Rect(0, 0, fallbackWidth, fallbackHeight)
	return &fallbackDisplay{
		bufs: [2]*image.RGBA{
			image.NewRGBA(rect),
			image.NewRGBA(rect),
		},
	}
}

func (d *fallbackDisplay) Width() int              { return fallbackWidth }
func (d *fallbackDisplay) Height() int             { return fallbackHeight }
func (d *fallbackDisplay) BackBuffer() *image.RGBA { return d.bufs[1-d.active] }
func (d *fallbackDisplay) Flip() error             { d.active = 1 - d.active; return nil }
func (d *fallbackDisplay) Blit(*image.RGBA)        {}
func (d *fallbackDisplay) Close()                  {}