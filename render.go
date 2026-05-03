// render.go draws a full display frame onto an RGBA image: stop header,
// upcoming bus arrivals with countdown times, weather, and a clock.
package main

import (
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log/slog"
	"os"
	"time"

	"golang.org/x/image/colornames"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed fonts/Anton-subset.ttf
var antonTTF []byte

const (
	slotHeight      = 130
	scrollDuration  = 5 * time.Second
	busListY        = 150 // Y baseline of the first bus row, below the header band
	maxVisibleBuses = 3   // bus rows shown when not animating
)

// smoothstep maps t ∈ [0,1] to [0,1] with zero first-derivative at both
// endpoints, producing an ease-in/ease-out curve: 3t² − 2t³.
// See https://en.wikipedia.org/wiki/Smoothstep
func smoothstep(t float64) float64 {
	return t * t * (3 - 2*t)
}

func newFaceFromBytes(data []byte, size float64) (xfont.Face, error) {
	ttf, err := opentype.Parse(data)
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

func newFace(size float64) (xfont.Face, error) {
	return newFaceFromBytes(gobold.TTF, size)
}

func newFaceFromFile(path string, size float64, fallback xfont.Face) (xfont.Face, error) {
	if path == "" {
		return fallback, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("could not load custom font", "path", path, "err", err)
		return fallback, nil
	}
	face, err := newFaceFromBytes(data, size)
	if err != nil {
		slog.Warn("could not parse custom font", "path", path, "err", err)
		return fallback, nil
	}
	return face, nil
}

func newAntonFace(size float64) (xfont.Face, error) {
	return newFaceFromBytes(antonTTF, size)
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
	prevTopETA  time.Time      // ETA of the bus drawn at top last frame; zero if none
	scrollStart time.Time      // when the current scroll animation started; zero if idle
	scrollBuses []Bus          // bus list snapshot taken when scroll started; nil when idle
	stopID      int            // stop currently being rendered; animation resets on change; -1 = clock screen
	bigFace     xfont.Face     // large face for bus numbers and ETAs
	smallFace   xfont.Face     // small face for header, footer, and clock
	clockFace   xfont.Face     // largest face that fits the screen width, used for clock screen
	border      int            // left/right margin in pixels; proportional to screen width
	fg          *image.Uniform // foreground (text) colour
	bg          *image.Uniform // background colour
}

// newClockFace returns the largest face whose rendered "00:00" fits within
// the available width (screen width minus both borders).
func newClockFace(width, border int) (xfont.Face, error) {
	available := width - 2*border
	for size := 360.0; size >= 20; size -= 2 {
		face, err := newAntonFace(size)
		if err != nil {
			return nil, err
		}
		if measureString(face, "00:00") <= available {
			slog.Debug("clock face", "size", size, "available", available)
			return face, nil
		}
		face.Close()
	}
	return newFace(20)
}

func parseColor(s string) *image.Uniform {
	if c, ok := colornames.Map[s]; ok {
		return image.NewUniform(c)
	}
	slog.Warn("could not parse color - defaulting to white", "color", s)
	return image.NewUniform(color.White)
}

func newRenderer(width int, invert bool, fontPath string, fontHeight int, textColor string) (*renderer, error) {
	bigFace, err := newFace(float64(fontHeight))
	if err != nil {
		return nil, err
	}
	smallFace, err := newFace(32)
	if err != nil {
		bigFace.Close()
		return nil, err
	}
	border := 80
	if width != 800 {
		border = 10
	}
	clockFace, err := newClockFace(width, border)
	if err != nil {
		bigFace.Close()
		smallFace.Close()
		return nil, err
	}

	// Load custom font for big face (bus numbers and times) if provided
	if fontPath != "" {
		customBigFace, err := newFaceFromFile(fontPath, float64(fontHeight), bigFace)
		if err == nil && customBigFace != bigFace {
			bigFace = customBigFace
		}
	}

	fg := image.NewUniform(color.White)
	bg := image.NewUniform(color.Black)

	if invert {
		fg = image.NewUniform(color.Black)
		bg = image.NewUniform(color.White)
	}
	if textColor != "" {
		fg = parseColor(textColor)
	}

	return &renderer{
		bigFace:   bigFace,
		smallFace: smallFace,
		clockFace: clockFace,
		border:    border,
		fg:        fg,
		bg:        bg,
	}, nil
}

func (r *renderer) close() {
	r.bigFace.Close()
	r.smallFace.Close()
	r.clockFace.Close()
}

// resetAnimation clears all scroll animation state and sets the current stop ID.
func (r *renderer) resetAnimation(stopID int) {
	r.stopID = stopID
	r.prevTopETA = time.Time{}
	r.scrollStart = time.Time{}
	r.scrollBuses = nil
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
		return firstIdx, busListY, maxVisibleBuses
	}
	startIdx = firstIdx
	if firstIdx > 0 {
		startIdx = firstIdx - 1
	}
	return startIdx, busListY - (slotHeight - yOffset), maxVisibleBuses + 1
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
		drawString(img, r.bigFace, r.border, y, b.Number, r.fg)
		var etaStr string
		if d < 0 {
			etaStr = "Due"
		} else {
			etaStr = d.String()
		}
		rightX := img.Bounds().Max.X - r.border - measureString(r.bigFace, etaStr)
		drawString(img, r.bigFace, rightX, y, etaStr, r.fg)
		y += slotHeight
		count++
	}
}

// drawFooter renders the weather string and clock onto the bottom of img.
func (r *renderer) drawFooter(img *image.RGBA, weatherStr string) {
	y := img.Bounds().Max.Y - r.smallFace.Metrics().Descent.Ceil()
	w := img.Bounds().Max.X
	drawString(img, r.smallFace, r.border, y, weatherStr, r.fg)
	timeStr := time.Now().Format("3:04:05")
	drawString(img, r.smallFace, w-r.border-measureString(r.smallFace, timeStr), y, timeStr, r.fg)
}

// drawHeader renders the stop name and direction onto the top of img.
func (r *renderer) drawHeader(img *image.RGBA, tt *timeTable) {
	info := tt.getStopInfo()
	header := info.Name
	if info.Towards != "" {
		header += " - To: " + info.Towards
	}
	if header != "" {
		drawString(img, r.smallFace, r.border, r.smallFace.Metrics().Ascent.Ceil(), header, r.fg)
	}
}

// renderClock draws a full-screen clock showing the time (in the largest font
// that fits), the date, and the current weather at the bottom.
func (r *renderer) renderClock(img *image.RGBA, weatherStr string) {
	draw.Draw(img, img.Bounds(), r.bg, image.Point{}, draw.Src)

	w := img.Bounds().Max.X
	h := img.Bounds().Max.Y

	now := time.Now()
	timeStr := now.Format("3:04")
	dateStr := now.Format("Monday 2 January 2006")

	clockAsc := r.clockFace.Metrics().Ascent.Ceil()
	clockDesc := r.clockFace.Metrics().Descent.Ceil()
	smallAsc := r.smallFace.Metrics().Ascent.Ceil()
	smallDesc := r.smallFace.Metrics().Descent.Ceil()

	const gap = 16

	// Anchor date and weather at the bottom, then centre the time in the
	// space that remains above.
	weatherY := h - smallDesc
	dateY := weatherY - (smallAsc + smallDesc) - gap

	clockSpace := dateY - smallAsc - gap
	timeY := (clockSpace-clockAsc-clockDesc)/2 + clockAsc

	drawString(img, r.clockFace, (w-measureString(r.clockFace, timeStr))/2, timeY, timeStr, r.fg)
	drawString(img, r.smallFace, (w-measureString(r.smallFace, dateStr))/2, dateY, dateStr, r.fg)
	drawString(img, r.smallFace, r.border, weatherY, weatherStr, r.fg)
}

// renderFrame draws a complete frame onto img. If tt is nil the clock screen
// is shown instead of bus arrivals.
func (r *renderer) renderFrame(img *image.RGBA, tt *timeTable, weatherStr string) {
	if tt == nil {
		if r.stopID != -1 {
			r.resetAnimation(-1)
		}
		r.renderClock(img, weatherStr)
		return
	}

	w := img.Bounds().Max.X
	h := img.Bounds().Max.Y

	// Reset animation state when the stop changes (e.g. user tapped to switch).
	if tt.stopID != r.stopID {
		r.resetAnimation(tt.stopID)
	}

	draw.Draw(img, img.Bounds(), r.bg, image.Point{}, draw.Src)

	buses, yOffset := r.advanceScroll(tt.getBuses(), time.Now())
	startIdx, startY, maxBuses := r.scrollLayout(firstIdxOf(buses), yOffset)
	r.drawBuses(img, buses, startIdx, startY, maxBuses)

	// Blank header and footer bands so scrolling bus rows cannot overwrite them.
	const headerH, footerH = 40, 40
	draw.Draw(img, image.Rect(0, 0, w, headerH), r.bg, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, h-footerH, w, h), r.bg, image.Point{}, draw.Src)

	// Header and footer drawn last so they always appear on top of bus rows.
	r.drawHeader(img, tt)
	r.drawFooter(img, weatherStr)

}
