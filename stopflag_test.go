package main

import (
	"image"
	"testing"

	"golang.org/x/image/colornames"
)

func TestStopListSet(t *testing.T) {
	var stops stopList
	for _, s := range []string{"123", "456:darkblue"} {
		err := stops.Set(s)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(stops) != 2 || stops[0].code != 123 || stops[0].bg != nil || stops[1].code != 456 {
		t.Fatalf("got %+v", stops)
	}
	if stops[1].bg.C != colornames.Darkblue {
		t.Errorf("got background %v, want darkblue", stops[1].bg.C)
	}
	for _, s := range []string{"", "abc", "123:", "123:nosuchcolor"} {
		err := stops.Set(s)
		if err == nil {
			t.Errorf("%q: expected error", s)
		}
	}
}

func TestRenderStopBackground(t *testing.T) {
	r := newTestRenderer(t)
	img := image.NewRGBA(image.Rect(0, 0, 800, 480))
	tt := newTimeTable(1)
	tt.bg = image.NewUniform(colornames.Darkblue)

	r.renderFrame(img, tt, "")

	// Sample the header band, the bus list and the footer band.
	for _, y := range []int{0, 240, 479} {
		got := img.RGBAAt(0, y)
		if got != colornames.Darkblue {
			t.Errorf("y=%d: got %v, want darkblue", y, got)
		}
	}
}
