// doublebuf.go implements double buffering for the display render loop.
package main

import (
	"image"
	"sync"
	"sync/atomic"
	"time"

	xfont "golang.org/x/image/font"
)

// frameBuffer uses double buffering: the render loop writes into the back
// buffer while readers (HTTP handlers) read from the front buffer.
// publishFrame swaps them. Both front and back are protected by mu.
type frameBuffer struct {
	mu    sync.RWMutex
	bufs  [2]image.RGBA
	front int // index of the front (display) buffer; protected by mu
	back  int // index of the back (render) buffer; protected by mu
}

func newFrameBuffer(width, height int) *frameBuffer {
	rect := image.Rect(0, 0, width, height)
	fb := &frameBuffer{front: 0, back: 1}
	fb.bufs[0] = *image.NewRGBA(rect)
	fb.bufs[1] = *image.NewRGBA(rect)
	return fb
}

// backBuf returns the buffer to draw into for the next frame.
func (fb *frameBuffer) backBuf() *image.RGBA {
	fb.mu.RLock()
	defer fb.mu.RUnlock()
	return &fb.bufs[fb.back]
}

// publishFrame swaps back and front, making the just-rendered frame
// visible to readers. Blocks if a reader is currently encoding the front buffer.
func (fb *frameBuffer) publishFrame() {
	fb.mu.Lock()
	fb.front, fb.back = fb.back, fb.front
	fb.mu.Unlock()
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
	r := newRenderer()
	wasAnimating := false
	for {
		back := buf.backBuf()
		animating := r.renderFrame(back, bigFace, smallFace, active.Load(), *weather.Load())
		hw.blit(back, rotate)
		buf.publishFrame()
		if animating != wasAnimating {
			if animating {
				tick.Reset(time.Millisecond)
			} else {
				tick.Reset(time.Second)
			}
			wasAnimating = animating
		}
		select {
		case <-tick.C:
		case <-notify:
		}
	}
}
