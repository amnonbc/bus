// doublebuf.go implements double buffering for the display render loop.
package main

import (
	"image"
	"sync"
	"sync/atomic"
	"time"
)

// frameBuffer uses double buffering: the render loop writes into the back
// buffer while readers (HTTP handlers) read from the front buffer.
// publishFrame swaps them. Both front and back are protected by mu.
// pool is a typed wrapper around sync.Pool to avoid type assertions at call sites.
type pool[T any] struct {
	p sync.Pool
}

func newPool[T any](newFn func() *T) pool[T] {
	return pool[T]{p: sync.Pool{New: func() any { return newFn() }}}
}

func (p *pool[T]) get() *T  { return p.p.Get().(*T) }
func (p *pool[T]) put(v *T) { p.p.Put(v) }

type frameBuffer struct {
	mu    sync.RWMutex
	bufs  [2]image.RGBA
	front int              // index of the front (display) buffer; protected by mu
	back  int              // index of the back (render) buffer; protected by mu
	pool  pool[image.RGBA] // recycles image snapshots for copyFront
	r     *renderer
	hw    blitter
}

func newFrameBuffer(width, height int, hw blitter, invert bool) (*frameBuffer, error) {
	r, err := newRenderer(width, invert)
	if err != nil {
		return nil, err
	}
	rect := image.Rect(0, 0, width, height)
	size := width * height * 4
	fb := &frameBuffer{
		front: 0,
		back:  1,
		pool:  newPool(func() *image.RGBA { return &image.RGBA{Pix: make([]byte, size)} }),
		r:     r,
		hw:    hw,
	}
	fb.bufs[0] = *image.NewRGBA(rect)
	fb.bufs[1] = *image.NewRGBA(rect)
	return fb, nil
}

func (fb *frameBuffer) close() {
	fb.r.close()
}

// backBuf returns the buffer to draw into for the next frame.
func (fb *frameBuffer) backBuf() *image.RGBA {
	fb.mu.RLock()
	defer fb.mu.RUnlock()
	return &fb.bufs[fb.back]
}

// copyFront returns a snapshot of the front buffer using a pooled pixel slice.
// The lock is held only for the pixel copy, not for any subsequent encoding.
// Call recycle when done to return the slice to the pool.
func (fb *frameBuffer) copyFront() *image.RGBA {
	fb.mu.RLock()
	src := &fb.bufs[fb.front]
	img := fb.pool.get()
	img.Stride = src.Stride
	img.Rect = src.Rect
	copy(img.Pix, src.Pix)
	fb.mu.RUnlock()
	return img
}

// recycle returns a copyFront snapshot to the pool.
func (fb *frameBuffer) recycle(img *image.RGBA) {
	fb.pool.put(img)
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
	Width() int
	Height() int
	Blit(img *image.RGBA)
}

type noopBlitter struct{}

func (noopBlitter) Width() int       { return 0 }
func (noopBlitter) Height() int      { return 0 }
func (noopBlitter) Blit(*image.RGBA) {}

// runLoop is the shared render loop used on all platforms. It renders a frame
// each tick (or immediately on notify), publishes it via double buffering for
// the HTTP preview, and passes it to hw for hardware display if provided.
func (fb *frameBuffer) runLoop(active *atomic.Pointer[timeTable], weather *atomic.Pointer[string], notify <-chan struct{}) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	wasAnimating := false
	for {
		back := fb.backBuf()
		renderStart := time.Now()
		fb.r.renderFrame(back, active.Load(), *weather.Load())
		metricFrameRender.Observe(time.Since(renderStart).Seconds())
		fb.hw.Blit(back)
		fb.publishFrame()
		if fb.r.isAnimating() != wasAnimating {
			if fb.r.isAnimating() {
				tick.Reset(33 * time.Millisecond) // ~30 fps during animation
			} else {
				tick.Reset(time.Second)
			}
			wasAnimating = fb.r.isAnimating()
		}
		select {
		case <-tick.C:
		case <-notify:
		}
	}
}
