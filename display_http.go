// display_http.go serves a live PNG preview of the display over HTTP on all
// platforms, along with pprof endpoints for profiling.
package main

import (
	_ "embed"
	"errors"
	"image/png"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"syscall"
)

//go:embed index.html
var htmlResponse []byte

//go:embed favicon.png
var faviconData []byte

type httpPreview struct {
	buf  *frameBuffer
	flip func()
}

func newHTTPPreview(buf *frameBuffer, flip func()) *httpPreview {
	return &httpPreview{buf: buf, flip: flip}
}

func (p *httpPreview) serveFrame(w http.ResponseWriter, r *http.Request) {
	snap := p.buf.copyFront()
	defer p.buf.recycle(snap)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	err := png.Encode(w, snap)
	if err != nil && !errors.Is(err, syscall.EPIPE) {
		slog.Error("png encode", "err", err)
	}
}

func (p *httpPreview) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write(htmlResponse)
}

func (p *httpPreview) serveFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Write(faviconData)
}

func (p *httpPreview) serveFlip(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if p.flip == nil {
		http.Error(w, "no second stop configured", http.StatusNotFound)
		return
	}
	p.flip()
	w.WriteHeader(http.StatusNoContent)
}

func (p *httpPreview) register() {
	http.HandleFunc("/frame.png", p.serveFrame)
	http.HandleFunc("/favicon.ico", p.serveFavicon)
	http.HandleFunc("/flip", p.serveFlip)
	http.HandleFunc("/", p.serveIndex)
}

// listenHTTP starts the HTTP server and logs any error on exit.
// Intended to be called as a goroutine when other work runs concurrently.
func listenHTTP() {
	err := http.ListenAndServe(":8080", nil)
	slog.Error("HTTP preview server", "err", err)
}
