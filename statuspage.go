// statuspage.go draws the status screen: the status messages of every stop,
// word-wrapped in the largest font that fits, so nothing needs to scroll.
package main

import (
	"image"
	"image/draw"
	"log/slog"
	"slices"
	"strings"

	xfont "golang.org/x/image/font"
)

const (
	maxStatusFontSize = 64
	minStatusFontSize = 14
	noStatusText      = "No status messages"
)

// newStatusScreen returns a pseudo timeTable which selects the status screen.
func newStatusScreen(stops []*timeTable) *timeTable {
	return &timeTable{statusFrom: stops}
}

// statusMessages returns the distinct status messages of the stops, with all
// runs of white space (including TFL's line breaks) collapsed to single spaces.
func statusMessages(stops []*timeTable) []string {
	var out []string
	for _, tt := range stops {
		for _, status := range tt.getStopInfo().Status {
			text := strings.Join(strings.Fields(status.Text), " ")
			if text != "" && !slices.Contains(out, text) {
				out = append(out, text)
			}
		}
	}
	return out
}

// wrapText splits text into lines no wider than width. A single word wider
// than width gets a line to itself.
func wrapText(face xfont.Face, text string, width int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		candidate := strings.TrimSpace(line + " " + word)
		if line != "" && measureString(face, candidate) > width {
			lines = append(lines, line)
			candidate = word
		}
		line = candidate
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// wrapMessages wraps each message, with an empty line between messages.
func wrapMessages(face xfont.Face, messages []string, width int) []string {
	var lines []string
	for i, m := range messages {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, wrapText(face, m, width)...)
	}
	return lines
}

// linesFit reports whether the lines fit within the given width and height.
func linesFit(face xfont.Face, lines []string, width, height int) bool {
	if len(lines)*face.Metrics().Height.Ceil() > height {
		return false
	}
	for _, line := range lines {
		if measureString(face, line) > width {
			return false
		}
	}
	return true
}

// statusLayout is a wrapped set of messages and the face to draw them with.
type statusLayout struct {
	key   string // messages the layout was computed for
	face  xfont.Face
	size  int // font size of face
	lines []string
}

// fitStatus returns a layout using the largest face for which the messages fit.
// If they do not fit even at the smallest size, the smallest size is used and
// the bottom lines are cut off.
func fitStatus(messages []string, width, height int) (statusLayout, error) {
	for size := maxStatusFontSize; ; size -= 2 {
		face, err := newFace(float64(size))
		if err != nil {
			return statusLayout{}, err
		}
		lines := wrapMessages(face, messages, width)
		if size <= minStatusFontSize || linesFit(face, lines, width, height) {
			return statusLayout{face: face, size: size, lines: lines}, nil
		}
		face.Close()
	}
}

// updateStatusLayout recomputes the cached layout if the messages have changed.
func (r *renderer) updateStatusLayout(messages []string, width, height int) {
	count := len(messages)
	if count == 0 {
		messages = []string{noStatusText}
	}
	key := strings.Join(messages, "\n")
	if r.status.face != nil && r.status.key == key {
		return
	}
	layout, err := fitStatus(messages, width, height)
	if err != nil {
		slog.Error("status layout", "err", err)
		return
	}
	if r.status.face != nil {
		r.status.face.Close()
	}
	layout.key = key
	r.status = layout
	slog.Info("status layout", "messages", count, "fontSize", layout.size, "lines", len(layout.lines))
}

// renderStatus draws the status screen, with the messages centred vertically.
func (r *renderer) renderStatus(img *image.RGBA, stops []*timeTable) {
	draw.Draw(img, img.Bounds(), r.bg, image.Point{}, draw.Src)

	messages := statusMessages(stops)
	width := img.Bounds().Max.X - 2*r.border
	height := img.Bounds().Max.Y
	r.updateStatusLayout(messages, width, height)
	if r.status.face == nil {
		return
	}

	metrics := r.status.face.Metrics()
	lineH := metrics.Height.Ceil()
	top := max((height-len(r.status.lines)*lineH)/2, 0)
	y := top + metrics.Ascent.Ceil()
	for _, line := range r.status.lines {
		drawString(img, r.status.face, r.border, y, line, orangeColor)
		y += lineH
	}
}
