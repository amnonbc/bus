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

// renderer holds animation state between frames.
type renderer struct {
	prevTopETA time.Time // ETA of the bus drawn at top last frame; zero if none
	scrollStart time.Time // when the current scroll animation started; zero if idle
}

func newRenderer() *renderer {
	return &renderer{}
}

// renderFrame draws a complete frame and returns true while a scroll animation
// is in progress. The caller should render at a high frame rate while true.
func (r *renderer) renderFrame(img *image.RGBA, bigFace, smallFace xfont.Face, tt *timeTable, weatherStr string) bool {
	const border = 20
	w := img.Bounds().Max.X
	h := img.Bounds().Max.Y

	draw.Draw(img, img.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)

	buses := tt.getBuses()

	// firstIdx is the index of the first bus that has not yet departed.
	firstIdx := 0
	for firstIdx < len(buses) && fromTime(buses[firstIdx].ETA) < 0 {
		firstIdx++
	}

	now := time.Now()

	// When the bus that was at the top has now departed, start scrolling.
	if !r.prevTopETA.IsZero() && r.scrollStart.IsZero() && fromTime(r.prevTopETA) < 0 {
		r.scrollStart = now
	}

	// Calculate the vertical offset: starts at slotHeight, eases to 0.
	animating := !r.scrollStart.IsZero()
	yOffset := 0
	if animating {
		elapsed := now.Sub(r.scrollStart)
		if elapsed >= scrollDuration {
			r.scrollStart = time.Time{}
			animating = false
		} else {
			t := float64(elapsed) / float64(scrollDuration)
			t = t * t * (3 - 2*t) // smoothstep
			yOffset = int(float64(slotHeight) * (1 - t))
		}
	}

	// Record the current top bus for next frame's departure check.
	if firstIdx < len(buses) {
		r.prevTopETA = buses[firstIdx].ETA
	} else {
		r.prevTopETA = time.Time{}
	}

	// During animation include the departing bus and draw one slot higher.
	startIdx := firstIdx
	startY := 150
	maxBuses := 3
	if animating {
		if firstIdx > 0 {
			startIdx = firstIdx - 1
		}
		startY = 150 - (slotHeight - yOffset)
		maxBuses = 4
	}

	// Draw buses first so the header and footer are painted on top.
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
		drawString(img, bigFace, border, y, b.Number, color.White)

		etaStr := d.String()
		if d < 0 {
			etaStr = "Due"
		}
		rightX := w - border - measureString(bigFace, etaStr)
		drawString(img, bigFace, rightX, y, etaStr, color.White)

		y += slotHeight
		count++
	}

	// Blank header and footer bands so scrolling bus rows cannot overwrite them.
	const headerH = 40
	const footerH = 40
	black := image.NewUniform(color.Black)
	draw.Draw(img, image.Rect(0, 0, w, headerH), black, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, h-footerH, w, h), black, image.Point{}, draw.Src)

	// Header and footer drawn last so they always appear on top of bus rows.
	grey := color.Gray{Y: 180}
	info := tt.getStopInfo()
	header := info.Name
	if info.Towards != "" {
		header += " - To: " + info.Towards
	}
	if header != "" {
		drawString(img, smallFace, border, 28, header, grey)
	}

	bottomY := h - 20
	drawString(img, smallFace, border, bottomY, weatherStr, color.White)

	timeStr := time.Now().Format("3:04:05")
	drawString(img, smallFace, w-border-measureString(smallFace, timeStr), bottomY, timeStr, color.White)

	return animating
}
