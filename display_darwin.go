//go:build darwin

package main

import (
	_ "embed"
	"image"
	"image/png"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
)

//go:embed index.html
var htmlResponse []byte

// Display dimensions match the real Pi screen.
const (
	displayWidth  = 800
	displayHeight = 480
)

func runDisplay(active *atomic.Pointer[timeTable], weather *atomic.Pointer[string], rotate bool) error {
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

	// mu serialises access to the font faces: opentype.Face has an internal
	// sfnt.Buffer that is mutated on every glyph operation, causing a data
	// race if two handlers render concurrently.
	var mu sync.Mutex

	// Render a fresh frame on every request so the browser always gets
	// the current state when you hit refresh (or auto-refresh fires).
	http.HandleFunc("/frame.png", func(w http.ResponseWriter, r *http.Request) {
		img := image.NewRGBA(image.Rect(0, 0, displayWidth, displayHeight))
		weatherStr := *weather.Load()
		mu.Lock()
		renderFrame(img, bigFace, smallFace, active.Load(), weatherStr)
		mu.Unlock()
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		err := png.Encode(w, img)
		if err != nil {
			slog.Error("png encode", "err", err)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(htmlResponse)
	})

	slog.Info("preview server", "url", "http://localhost:8080")
	return http.ListenAndServe(":8080", nil)
}
