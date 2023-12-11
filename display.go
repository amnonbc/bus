package main

import (
	"flag"
	"log"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
)

func tm() string {
	return time.Now().Format("3:04:05")
}

func main() {
	stop := flag.Int("stop", 74640, "bus stop code")
	flag.Parse()

	myApp := app.New()
	myWindow := myApp.NewWindow("List Data")

	bottomRight := text(tm())
	bottomRight.TextSize = 40
	bottomRight.Alignment = fyne.TextAlignTrailing

	bottomLeft := text("weather")
	bottomLeft.TextSize = 32
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
	tick := time.NewTicker(time.Hour)
	for range tick.C {
		weatherUpdate(w)
	}
}

func weatherUpdate(w *canvas.Text) {
	weather, err := GetWeather()
	if err != nil {
		log.Println("could not get weather", err)
		w.Text = "Weather Error"
	} else if len(weather) > 0 {
		w.Text = weather[0].String()
	}
	w.Refresh()
}
