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
	"sync"
	"syscall"
)

//go:embed index.html
var htmlResponse []byte

// httpPreview uses double buffering: the render loop writes into the back
// buffer while the HTTP handler reads from the front buffer. publishFrame
// swaps them. bufs[front] is protected by mu; bufs[back] is owned
// exclusively by the render goroutine and needs no lock.
type httpPreview struct {
	mu    sync.RWMutex
	bufs  [2]image.RGBA
	front int // index of the front (display) buffer; protected by mu
	back  int // index of the back (render) buffer; only touched by render goroutine
}

func newHTTPPreview(width, height int) *httpPreview {
	rect := image.Rect(0, 0, width, height)
	p := &httpPreview{front: 0, back: 1}
	p.bufs[0] = *image.NewRGBA(rect)
	p.bufs[1] = *image.NewRGBA(rect)
	return p
}

// backBuf returns the buffer the render loop should draw into.
// Must only be called from the render goroutine.
func (p *httpPreview) backBuf() *image.RGBA {
	return &p.bufs[p.back]
}

// publishFrame completes the double-buffer swap: it promotes the back buffer
// to front (making it visible to HTTP handlers) and reclaims the old front
// as the new back for the next render. Must only be called from the render goroutine.
func (p *httpPreview) publishFrame() {
	newFront := p.back
	p.mu.Lock()
	p.front = newFront
	p.mu.Unlock()
	p.back = 1 - newFront
}

func (p *httpPreview) serveFrame(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	p.mu.RLock()
	err := png.Encode(w, &p.bufs[p.front])
	p.mu.RUnlock()
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
