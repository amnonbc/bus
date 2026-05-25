// doublebuf.go implements double buffering for the display render loop.
package main

import (
	"image"
	"log/slog"
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

type fbOptions struct {
	rotate     bool
	debug      bool
	forceFB    bool
	invert     bool
	fontPath   string
	fontHeight int
	textColor  string
	async      bool
}

type frameBuffer struct {
	mu    sync.RWMutex
	front *image.RGBA      // pointer to the front (display) buffer; protected by mu
	pool  pool[image.RGBA] // recycles image snapshots for copyFront
	r     *renderer
	hw    blitter
}

func newFrameBuffer(width, height int, hw blitter, opts fbOptions) (*frameBuffer, error) {
	r, err := newRenderer(width, opts.invert, opts.fontPath, opts.fontHeight, opts.textColor)
	if err != nil {
		return nil, err
	}
	size := width * height * 4
	fb := &frameBuffer{
		// Initialize front to the current back buffer. On the first iteration of runLoop,
		// we'll render into this buffer and flip it (making it the new front), so we start
		// in a consistent state. This pointer is updated by publishFrame() on each frame.
		front: hw.BackBuffer(),
		pool:  newPool(func() *image.RGBA { return &image.RGBA{Pix: make([]byte, size)} }),
		r:     r,
		hw:    hw,
	}
	return fb, nil
}

func (fb *frameBuffer) close() {
	fb.r.close()
}

// copyFront returns a snapshot of the front buffer using a pooled pixel slice.
// The lock is held only for the pixel copy, not for any subsequent encoding.
// Call recycle when done to return the slice to the pool.
func (fb *frameBuffer) copyFront() *image.RGBA {
	fb.mu.RLock()
	src := fb.front
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

// publishFrame updates the front buffer pointer, making the just-rendered frame
// visible to readers. Blocks if a reader is currently encoding the front buffer.
func (fb *frameBuffer) publishFrame(back *image.RGBA) {
	fb.mu.Lock()
	fb.front = back
	fb.mu.Unlock()
}

// blitter writes a rendered frame to a hardware display (e.g. framebuffer).
type blitter interface {
	Width() int
	Height() int
	BackBuffer() *image.RGBA
	Flip() error
}

// runLoop is the shared render loop used on all platforms. It renders a frame
// each tick (or immediately on notify), publishes it via double buffering for
// the HTTP preview, and passes it to hw for hardware display if provided.
func (fb *frameBuffer) runLoop(active *atomic.Pointer[timeTable], weather *atomic.Pointer[string], notify <-chan struct{}) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	wasAnimating := false
	for {
		back := fb.hw.BackBuffer()
		renderStart := time.Now()
		fb.r.renderFrame(back, active.Load(), *weather.Load())
		metricFrameRender.Observe(time.Since(renderStart).Seconds())
		err := fb.hw.Flip()
		if err != nil {
			slog.Error("flip failed", "err", err)
		}
		fb.publishFrame(back)
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
