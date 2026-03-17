// display_http.go serves a live PNG preview of the display over HTTP on all
// platforms, along with pprof endpoints for profiling.
package main

import (
	_ "embed"
	"errors"
	"image"
	"image/png"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"sync/atomic"
	"syscall"
	"time"

	xfont "golang.org/x/image/font"
)

//go:embed index.html
var htmlResponse []byte

type httpPreview struct {
	buf *frameBuffer
}

func newHTTPPreview(buf *frameBuffer) *httpPreview {
	return &httpPreview{buf: buf}
}

func (p *httpPreview) serveFrame(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	p.buf.mu.RLock()
	err := png.Encode(w, &p.buf.bufs[p.buf.front])
	p.buf.mu.RUnlock()
	if err != nil && !errors.Is(err, syscall.EPIPE) {
		slog.Error("png encode", "err", err)
	}
}

func (p *httpPreview) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write(htmlResponse)
}

func (p *httpPreview) register() {
	http.HandleFunc("/frame.png", p.serveFrame)
	http.HandleFunc("/", p.serveIndex)
}

// listenHTTP starts the HTTP server and logs any error on exit.
// Intended to be called as a goroutine when other work runs concurrently.
func listenHTTP() {
	err := http.ListenAndServe(":8080", nil)
	slog.Error("HTTP preview server", "err", err)
}

// blitter writes a rendered frame to a hardware display (e.g. framebuffer).
// noopBlitter is used on platforms with no physical display.
type blitter interface {
	blit(img *image.RGBA, rotate bool)
}

type noopBlitter struct{}

func (noopBlitter) blit(*image.RGBA, bool) {}

// runLoop is the shared render loop used on all platforms. It renders a frame
// each tick (or immediately on notify), publishes it via double buffering for
// the HTTP preview, and passes it to hw for hardware display if provided.
func runLoop(buf *frameBuffer, active *atomic.Pointer[timeTable], weather *atomic.Pointer[string], bigFace, smallFace xfont.Face, hw blitter, rotate bool, notify <-chan struct{}) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
		case <-notify:
		}
		renderFrame(buf.backBuf(), bigFace, smallFace, active.Load(), *weather.Load())
		hw.blit(buf.backBuf(), rotate)
		buf.publishFrame()
	}
}
