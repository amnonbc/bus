package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"runtime"
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

func loop(w *fyne.Container, busses *[]Bus) {

	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		updateBuses(w, *busses)
	}
}

func fmtDelay(delay time.Duration) string {
	s := int(delay.Seconds())
	m := s / 60
	s -= 60 * m
	return fmt.Sprintf("%d:%02d", m, s)
}

func updateBuses(c *fyne.Container, busses []Bus) {
	var w []fyne.CanvasObject
	for _, b := range busses {
		if len(w) > 4 {
			break
		}
		sb(b.Number)
		delay := time.Until(b.ETA).Round(time.Second)
		if delay < 0 {
			continue
		}

		w = append(w, sb(b.Number))
		eta := sb(fmtDelay(delay))
		eta.Alignment = fyne.TextAlignTrailing
		w = append(w, eta)
	}
	c.RemoveAll()
	for _, ww := range w {
		c.Add(ww)
	}
	c.Refresh()
}

const tflBase = "http://countdown.api.tfl.gov.uk/interfaces/ura/instant_V1"

func tm() string {
	return time.Now().Format("3:04:05")
}

func main() {
	stop := flag.Int("stop", 74640, "bus stop code")
	flag.Parse()

	myApp := app.New()
	myWindow := myApp.NewWindow("List Data")
	busses, err := GetCountdownData(tflBase, *stop)
	if err != nil {
		panic(err)
	}

	ll := container.New(
		layout.NewGridLayout(2),
	)
	bottomRight := sb(tm())
	bottomRight.TextSize = 40
	bottomRight.Alignment = fyne.TextAlignTrailing

	w, err := GetWeather()
	if err != nil {
		log.Println(err)
	}
	bottomLeft := sb(w.String())
	bottomLeft.TextSize = 40

	bottom := container.New(
		layout.NewGridLayout(2), bottomLeft, bottomRight,
	)

	go weatherUpdate(bottomLeft)

	go clockUpdate(bottomRight)
	border := container.New(layout.NewBorderLayout(
		layout.NewSpacer(), bottom,
		layout.NewSpacer(), layout.NewSpacer()), ll, bottom)
	myWindow.SetContent(border)

	go loop(ll, &busses)

	go func() {
		tick := time.NewTicker(30 * time.Second)
		for range tick.C {
			busses, err = GetCountdownData(tflBase, *stop)
			if err != nil {
				log.Println(err)
			}
		}
	}()
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

func weatherUpdate(w *canvas.Text) {
	tick := time.NewTicker(30 * time.Minute)
	for range tick.C {
		weather, err := GetWeather()
		if err != nil {
			w.Text = err.Error()
		} else {
			w.Text = weather.String()
		}
		w.Refresh()
	}
}
