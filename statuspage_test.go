package main

import (
	"bytes"
	"image"
	"log/slog"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

func TestStatusMessages(t *testing.T) {
	a, b := newTimeTable(1), newTimeTable(2)
	a.info.Store(&StopInfo{Status: []StatusMessage{{Text: "Stop  closed\nuse next stop"}}})
	b.info.Store(&StopInfo{Status: []StatusMessage{{Text: "Stop closed use next stop"}, {Text: " "}, {Text: "Diversion"}}})
	got := statusMessages([]*timeTable{a, b})
	want := []string{"Stop closed use next stop", "Diversion"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFitStatus(t *testing.T) {
	const width, height = 640, 480
	long := strings.Repeat("Buses are on diversion due to roadworks. ", 8)
	messages := []string{long, "Stop closed"}
	layout, err := fitStatus(messages, width, height)
	if err != nil {
		t.Fatal(err)
	}
	defer layout.face.Close()
	if !linesFit(layout.face, layout.lines, width, height) {
		t.Error("messages do not fit")
	}
	got := strings.Fields(strings.Join(layout.lines, " "))
	want := strings.Fields(strings.Join(messages, " "))
	if !slices.Equal(got, want) {
		t.Errorf("words lost: got %q", got)
	}
}

func TestWrapText(t *testing.T) {
	face, err := newFace(20)
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()
	width := measureString(face, "one two")
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"empty", "", nil},
		{"fits", "one two", []string{"one two"}},
		{"wraps", "one two one two", []string{"one two", "one two"}},
		{"long word", "a extraordinarily b", []string{"a", "extraordinarily", "b"}},
	}
	for _, tt := range tests {
		got := wrapText(face, tt.text, width)
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestFitStatusSize(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		wantSize int
	}{
		{"short uses largest", "Stop closed", maxStatusFontSize},
		{"too long uses smallest", strings.Repeat("diversion ", 2000), minStatusFontSize},
	}
	for _, tt := range tests {
		layout, err := fitStatus([]string{tt.message}, 640, 480)
		if err != nil {
			t.Fatal(err)
		}
		layout.face.Close()
		if layout.size != tt.wantSize {
			t.Errorf("%s: got size %d, want %d", tt.name, layout.size, tt.wantSize)
		}
	}
}

// captureLogs redirects the default logger to a buffer for the rest of the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func newTestRenderer(t *testing.T) *renderer {
	r, err := newRenderer(800, false, "", 120, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.close)
	return r
}

func TestStatusLayoutLogging(t *testing.T) {
	r := newTestRenderer(t)
	logs := captureLogs(t)

	r.updateStatusLayout([]string{"Stop closed", "Diversion"}, 640, 480)
	want := `msg="status layout" messages=2 fontSize=64 lines=3`
	if !strings.Contains(logs.String(), want) {
		t.Errorf("got log %q, want %q", logs, want)
	}

	logs.Reset()
	r.updateStatusLayout([]string{"Stop closed", "Diversion"}, 640, 480)
	if logs.Len() != 0 {
		t.Errorf("unchanged messages logged again: %q", logs)
	}

	r.updateStatusLayout([]string{"Diversion"}, 640, 480)
	want = "messages=1 "
	if !strings.Contains(logs.String(), want) {
		t.Errorf("got log %q, want %q", logs, want)
	}
}

func TestRenderStatusNoMessages(t *testing.T) {
	r := newTestRenderer(t)
	logs := captureLogs(t)
	img := image.NewRGBA(image.Rect(0, 0, 800, 480))

	r.renderFrame(img, newStatusScreen([]*timeTable{newTimeTable(1)}), "")

	want := []string{noStatusText}
	if !slices.Equal(r.status.lines, want) {
		t.Errorf("got lines %q, want %q", r.status.lines, want)
	}
	if !strings.Contains(logs.String(), "messages=0 ") {
		t.Errorf("got log %q, want messages=0", logs)
	}
	if r.needsFastFrames() {
		t.Error("status screen should not need fast frames")
	}
}

func TestAdvanceCyclesScreens(t *testing.T) {
	stop := newTimeTable(1)
	status := newStatusScreen([]*timeTable{stop})
	screens := []*timeTable{stop, status, nil}
	var idx atomic.Int32
	var active atomic.Pointer[timeTable]
	notify := make(chan struct{}, 1)
	captureLogs(t)

	for i, want := range []*timeTable{status, nil, stop, status} {
		advance(screens, &idx, &active, notify)
		if got := active.Load(); got != want {
			t.Errorf("flip %d: got %p, want %p", i, got, want)
		}
	}
}
