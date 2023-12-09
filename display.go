package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"runtime"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
)

func sb(s string) *canvas.Text {
	colour := color.Black
	if runtime.GOOS == "linux" {
		colour = color.White
	}
	c := canvas.NewText(s, colour)
	c.TextSize = 100
	c.TextStyle.Bold = true
	return c
}

type timeTable struct {
	sync.Mutex
	stopID int
	c      *fyne.Container
	busses []Bus
}

func newTimeTable(stopID int) *timeTable {
	c := container.New(
		layout.NewGridLayout(2),
	)
	return &timeTable{
		stopID: stopID,
		c:      c}
}

func (t *timeTable) start() {
	t.download()
	go t.displayLoop()
	go t.downloadLoop()
}

func (t *timeTable) displayLoop() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		t.draw()
	}
}

type delay time.Duration

func (d delay) String() string {
	s := int(time.Duration(d).Seconds())
	m := s / 60
	s -= 60 * m
	return fmt.Sprintf("%d:%02d", m, s)
}

func fromTime(t time.Time) delay {
	return delay(time.Until(t).Round(time.Second))
}

func (t *timeTable) draw() {
	var w []fyne.CanvasObject
	t.Lock()
	defer t.Unlock()
	for _, b := range t.busses {
		if len(w) > 4 {
			break
		}
		delay := fromTime(b.ETA)
		if delay < 0 {
			continue
		}

		w = append(w, sb(b.Number))
		eta := sb(delay.String())
		eta.Alignment = fyne.TextAlignTrailing
		w = append(w, eta)
	}
	t.c.RemoveAll()
	for _, ww := range w {
		t.c.Add(ww)
	}
	t.c.Refresh()
}

func (t *timeTable) download() {
	b, err := GetCountdownData(tflBase, t.stopID)
	if err != nil {
		log.Println(err)
		return
	}
	t.Lock()
	t.busses = b
	t.Unlock()
}

func (t *timeTable) downloadLoop() {
	tick := time.NewTicker(30 * time.Second)
	for range tick.C {
		t.download()
	}
}

func tm() string {
	return time.Now().Format("3:04:05")
}

func main() {
	stop := flag.Int("stop", 74640, "bus stop code")
	flag.Parse()

	myApp := app.New()
	myWindow := myApp.NewWindow("List Data")

	bottomRight := sb(tm())
	bottomRight.TextSize = 40
	bottomRight.Alignment = fyne.TextAlignTrailing

	bottomLeft := sb("weather")
	bottomLeft.TextSize = 40
	weatherUpdate(bottomLeft)

	bottom := container.New(
		layout.NewGridLayout(2), bottomLeft, bottomRight,
	)

	go weatherLoop(bottomLeft)

	go clockUpdate(bottomRight)

	centre := newTimeTable(*stop)
	centre.start()

	border := container.New(layout.NewBorderLayout(
		layout.NewSpacer(), bottom,
		layout.NewSpacer(), layout.NewSpacer()), centre.c, bottom)
	myWindow.SetContent(border)

	go centre.displayLoop()

	if runtime.GOOS == "linux" {
		myWindow.SetFullScreen(true)
	}
	myWindow.ShowAndRun()
}

func clockUpdate(bottom *canvas.Text) {
	tick := time.NewTicker(time.Second)
	for range tick.C {
		bottom.Text = tm()
		bottom.Refresh()
	}
}

func weatherLoop(w *canvas.Text) {
	tick := time.NewTicker(30 * time.Minute)
	for range tick.C {
		weatherUpdate(w)
	}
}

func weatherUpdate(w *canvas.Text) {
	weather, err := GetWeather()
	if err != nil {
		w.Text = err.Error()
	} else if len(weather) > 0 {
		w.Text = weather[0].String()
	}
	w.Refresh()
}
