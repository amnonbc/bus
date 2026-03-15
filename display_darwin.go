//go:build darwin

package main

import (
	"image"
	"image/png"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
)

// Display dimensions match the real Pi screen.
const (
	displayWidth  = 800
	displayHeight = 480
)

func runDisplay(tt *timeTable, weather *atomic.Pointer[string], rotate bool) error {
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
		renderFrame(img, bigFace, smallFace, tt, weatherStr)
		mu.Unlock()
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		if err := png.Encode(w, img); err != nil {
			slog.Error("png encode", "err", err)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Bus times</title></head>
<body style="background:#111;margin:0;display:flex;justify-content:center;align-items:center;height:100vh">
<img id="f" src="/frame.png" style="image-rendering:pixelated;max-width:100%">
<script>
setInterval(function(){
    document.getElementById('f').src = '/frame.png?' + Date.now();
}, 1000);
</script>
</body>
</html>`))
	})

	slog.Info("preview server", "url", "http://localhost:8080")
	return http.ListenAndServe(":8080", nil)
}
