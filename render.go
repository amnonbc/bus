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
	border         = 80
)

// smoothstep maps t ∈ [0,1] to [0,1] with zero first-derivative at both
// endpoints, producing an ease-in/ease-out curve: 3t² − 2t³.
// See https://en.wikipedia.org/wiki/Smoothstep
func smoothstep(t float64) float64 {
	return t * t * (3 - 2*t)
}

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

func drawString(img *image.RGBA, face xfont.Face, x, y int, s string, clr *image.Uniform) {
	d := xfont.Drawer{
		Dst:  img,
		Src:  clr,
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
	prevTopETA  time.Time  // ETA of the bus drawn at top last frame; zero if none
	scrollStart time.Time  // when the current scroll animation started; zero if idle
	scrollBuses []Bus      // bus list snapshot taken when scroll started; nil when idle
	stopID      int        // stop currently being rendered; animation resets on change
	bigFace     xfont.Face // large face for bus numbers and ETAs
	smallFace   xfont.Face // small face for header, footer, and clock
}

func newRenderer() (*renderer, error) {
	bigFace, err := newFace(100)
	if err != nil {
		return nil, err
	}
	smallFace, err := newFace(32)
	if err != nil {
		bigFace.Close()
		return nil, err
	}
	return &renderer{bigFace: bigFace, smallFace: smallFace}, nil
}

func (r *renderer) close() {
	r.bigFace.Close()
	r.smallFace.Close()
}

// isAnimating reports whether a scroll animation is currently in progress.
func (r *renderer) isAnimating() bool {
	return !r.scrollStart.IsZero()
}

// advanceScroll updates animation state and returns the bus slice to draw from
// and the current vertical pixel offset.
// liveBuses is always used for prevTopETA so the next departure is detected
// correctly; the returned slice is the snapshot during animation.
func (r *renderer) advanceScroll(liveBuses []Bus, now time.Time) (buses []Bus, yOffset int) {
	liveFirstIdx := firstIdxOf(liveBuses)

	// Detect departure of the top bus → start scroll and snapshot the list.
	if !r.prevTopETA.IsZero() && r.scrollStart.IsZero() && fromTime(r.prevTopETA) < 0 {
		r.scrollStart = now
		r.scrollBuses = liveBuses
	}

	// Advance or end the animation.
	if r.isAnimating() {
		elapsed := time.Since(r.scrollStart)
		if elapsed >= scrollDuration {
			r.scrollStart = time.Time{}
			r.scrollBuses = nil
		} else {
			// t runs 0→1 over scrollDuration. yOffset starts at slotHeight
			// (buses appear one row below) and eases to 0 (final position),
			// sliding the list upward with a smooth ease-in/ease-out curve.
			t := float64(elapsed) / float64(scrollDuration)
			yOffset = int(float64(slotHeight) * (1 - smoothstep(t)))
		}
	}

	// Record the live top bus for next frame's departure check.
	if liveFirstIdx < len(liveBuses) {
		r.prevTopETA = liveBuses[liveFirstIdx].ETA
	} else {
		r.prevTopETA = time.Time{}
	}

	if r.isAnimating() {
		return r.scrollBuses, yOffset
	}
	return liveBuses, 0
}

// scrollLayout returns the drawing parameters for the bus list.
func (r *renderer) scrollLayout(firstIdx, yOffset int) (startIdx, startY, maxBuses int) {
	if !r.isAnimating() {
		return firstIdx, 150, 3
	}
	startIdx = firstIdx
	if firstIdx > 0 {
		startIdx = firstIdx - 1
	}
	return startIdx, 150 - (slotHeight - yOffset), 4
}

// drawBuses renders the bus arrival rows onto img.
func (r *renderer) drawBuses(img *image.RGBA, buses []Bus, startIdx, startY, maxBuses int) {
	y := startY
	count := 0
	for _, b := range buses[startIdx:] {
		if count >= maxBuses {
			break
		}
		d := fromTime(b.ETA)
		if d < 0 && !r.isAnimating() {
			continue
		}
		drawString(img, r.bigFace, border, y, b.Number, image.White)
		etaStr := d.String()
		if d < 0 {
			etaStr = "Due"
		}
		rightX := img.Bounds().Max.X - border - measureString(r.bigFace, etaStr)
		drawString(img, r.bigFace, rightX, y, etaStr, image.White)
		y += slotHeight
		count++
	}
}

// drawFooter renders the weather string and clock onto the bottom of img.
func (r *renderer) drawFooter(img *image.RGBA, weatherStr string) {
	y := img.Bounds().Max.Y - r.smallFace.Metrics().Descent.Ceil()
	w := img.Bounds().Max.X
	drawString(img, r.smallFace, border, y, weatherStr, image.White)
	timeStr := time.Now().Format("3:04:05")
	drawString(img, r.smallFace, w-border-measureString(r.smallFace, timeStr), y, timeStr, image.White)
}

var headerColour = image.NewUniform(color.Gray{Y: 180})

// drawHeader renders the stop name and direction onto the top of img.
func (r *renderer) drawHeader(img *image.RGBA, tt *timeTable) {
	info := tt.getStopInfo()
	header := info.Name
	if info.Towards != "" {
		header += " - To: " + info.Towards
	}
	if header != "" {
		drawString(img, r.smallFace, border, r.smallFace.Metrics().Ascent.Ceil(), header, headerColour)
	}
}

// renderFrame draws a complete frame onto img.
func (r *renderer) renderFrame(img *image.RGBA, tt *timeTable, weatherStr string) {
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

	buses, yOffset := r.advanceScroll(tt.getBuses(), time.Now())
	startIdx, startY, maxBuses := r.scrollLayout(firstIdxOf(buses), yOffset)
	r.drawBuses(img, buses, startIdx, startY, maxBuses)

	// Blank header and footer bands so scrolling bus rows cannot overwrite them.
	const headerH, footerH = 40, 40
	black := image.NewUniform(color.Black)
	draw.Draw(img, image.Rect(0, 0, w, headerH), black, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, h-footerH, w, h), black, image.Point{}, draw.Src)

	// Header and footer drawn last so they always appear on top of bus rows.
	r.drawHeader(img, tt)
	r.drawFooter(img, weatherStr)

}
