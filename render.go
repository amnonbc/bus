// render.go draws a full display frame onto an RGBA image: stop header,
// upcoming bus arrivals with countdown times, weather, and a clock.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	slotHeight     = 130
	scrollDuration = 5 * time.Second
)

func newFace(size float64) (xfont.Face, error) {
	ttf, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: xfont.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("new face: %w", err)
	}
	return face, nil
}

func measureString(face xfont.Face, s string) int {
	d := xfont.Drawer{Face: face}
	return int(d.MeasureString(s) >> 6)
}

func drawString(img *image.RGBA, face xfont.Face, x, y int, s string, clr color.Color) {
	d := xfont.Drawer{
		Dst:  img,
		Src:  image.NewUniform(clr),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

// firstIdxOf returns the index of the first bus in buses that has not yet departed.
func firstIdxOf(buses []Bus) int {
	for i, b := range buses {
		if fromTime(b.ETA) >= 0 {
			return i
		}
	}
	return len(buses)
}

// renderer holds animation state between frames.
type renderer struct {
	prevTopETA  time.Time // ETA of the bus drawn at top last frame; zero if none
	scrollStart time.Time // when the current scroll animation started; zero if idle
	scrollBuses []Bus     // bus list snapshot taken when scroll started; nil when idle
	stopID      int       // stop currently being rendered; animation resets on change
}

func newRenderer() *renderer {
	return &renderer{}
}

// advanceScroll updates animation state and returns the bus slice to draw from,
// the current vertical pixel offset, and whether animation is in progress.
// liveBuses is always used for prevTopETA so the next departure is detected
// correctly; the returned slice is the snapshot during animation.
func (r *renderer) advanceScroll(liveBuses []Bus, now time.Time) (buses []Bus, yOffset int, animating bool) {
	liveFirstIdx := firstIdxOf(liveBuses)

	// Detect departure of the top bus → start scroll and snapshot the list.
	if !r.prevTopETA.IsZero() && r.scrollStart.IsZero() && fromTime(r.prevTopETA) < 0 {
		r.scrollStart = now
		r.scrollBuses = liveBuses
	}

	// Advance or end the animation.
	animating = !r.scrollStart.IsZero()
	if animating {
		elapsed := time.Since(r.scrollStart)
		if elapsed >= scrollDuration {
			animating = false
			r.scrollStart = time.Time{}
			r.scrollBuses = nil
		} else {
			t := float64(elapsed) / float64(scrollDuration)
			t = t * t * (3 - 2*t) // smoothstep
			yOffset = int(float64(slotHeight) * (1 - t))
		}
	}

	// Record the live top bus for next frame's departure check.
	if liveFirstIdx < len(liveBuses) {
		r.prevTopETA = liveBuses[liveFirstIdx].ETA
	} else {
		r.prevTopETA = time.Time{}
	}

	if animating {
		return r.scrollBuses, yOffset, true
	}
	return liveBuses, 0, false
}

// scrollLayout returns the drawing parameters for the bus list.
func scrollLayout(firstIdx, yOffset int, animating bool) (startIdx, startY, maxBuses int) {
	if !animating {
		return firstIdx, 150, 3
	}
	startIdx = firstIdx
	if firstIdx > 0 {
		startIdx = firstIdx - 1
	}
	return startIdx, 150 - (slotHeight - yOffset), 4
}

// drawBuses renders the bus arrival rows onto img.
func drawBuses(img *image.RGBA, face xfont.Face, buses []Bus, startIdx, startY, maxBuses, w, border int, animating bool) {
	y := startY
	count := 0
	for _, b := range buses[startIdx:] {
		if count >= maxBuses {
			break
		}
		d := fromTime(b.ETA)
		if d < 0 && !animating {
			continue
		}
		drawString(img, face, border, y, b.Number, color.White)
		etaStr := d.String()
		if d < 0 {
			etaStr = "Due"
		}
		rightX := w - border - measureString(face, etaStr)
		drawString(img, face, rightX, y, etaStr, color.White)
		y += slotHeight
		count++
	}
}

// drawFooter renders the weather string and clock onto the bottom of img.
func drawFooter(img *image.RGBA, face xfont.Face, weatherStr string, w, h, border int) {
	y := h - 20
	drawString(img, face, border, y, weatherStr, color.White)
	timeStr := time.Now().Format("3:04:05")
	drawString(img, face, w-border-measureString(face, timeStr), y, timeStr, color.White)
}

// renderFrame draws a complete frame and returns true while a scroll animation
// is in progress. The caller should render at a high frame rate while true.
func (r *renderer) renderFrame(img *image.RGBA, bigFace, smallFace xfont.Face, tt *timeTable, weatherStr string) bool {
	const border = 20
	w := img.Bounds().Max.X
	h := img.Bounds().Max.Y

	// Reset animation state when the stop changes (e.g. user tapped to switch).
	if tt.stopID != r.stopID {
		r.stopID = tt.stopID
		r.prevTopETA = time.Time{}
		r.scrollStart = time.Time{}
		r.scrollBuses = nil
	}

	draw.Draw(img, img.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)

	buses, yOffset, animating := r.advanceScroll(tt.getBuses(), time.Now())
	startIdx, startY, maxBuses := scrollLayout(firstIdxOf(buses), yOffset, animating)
	drawBuses(img, bigFace, buses, startIdx, startY, maxBuses, w, border, animating)

	// Blank header and footer bands so scrolling bus rows cannot overwrite them.
	const headerH, footerH = 40, 40
	black := image.NewUniform(color.Black)
	draw.Draw(img, image.Rect(0, 0, w, headerH), black, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, h-footerH, w, h), black, image.Point{}, draw.Src)

	// Header and footer drawn last so they always appear on top of bus rows.
	info := tt.getStopInfo()
	header := info.Name
	if info.Towards != "" {
		header += " - To: " + info.Towards
	}
	if header != "" {
		drawString(img, smallFace, border, 28, header, color.Gray{Y: 180})
	}
	drawFooter(img, smallFace, weatherStr, w, h, border)

	return animating
}
