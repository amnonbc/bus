// doublebuf.go implements double buffering for the display render loop.
package main

import (
	"image"
	"sync"
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
	fb.mu.Lock()
	defer fb.mu.Unlock()
	return &fb.bufs[fb.back]
}

// publishFrame swaps back and front, making the just-rendered frame
// visible to readers. Blocks if a reader is currently encoding the front buffer.
func (fb *frameBuffer) publishFrame() {
	fb.mu.Lock()
	fb.front, fb.back = fb.back, fb.front
	fb.mu.Unlock()
}
