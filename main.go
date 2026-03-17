// main is the entry point. It parses flags, starts background goroutines for
// bus data, weather, and touch input, then runs the framebuffer display loop.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

const weatherURL = "https://api.weatherapi.com/v1/current.json"

// switchStop toggles active between tt1 and tt2 and notifies the render loop.
func switchStop(tt1, tt2 *timeTable, active *atomic.Pointer[timeTable], notify chan<- struct{}) {
	next := tt2
	if active.Load() == tt2 {
		next = tt1
	}
	active.Store(next)
	info := next.getStopInfo()
	slog.Info("switched bus stop", "stop", info.Name, "towards", info.Towards)
	select {
	case notify <- struct{}{}:
	default:
	}
}

func fetchWeather(apiKey, location string, weather *atomic.Pointer[string]) {
	w, err := GetWeather(weatherURL, apiKey, location)
	var s string
	if err != nil {
		slog.Error("weather", "err", err)
		s = "Weather Error"
	} else {
		s = w.String()
	}
	weather.Store(&s)
}

func weatherLoop(apiKey string, tt *timeTable, weather *atomic.Pointer[string]) {
	info := tt.getStopInfo()
	for info.Lat == 0 && info.Lon == 0 {
		slog.Warn("stop location not yet available, retrying weather in 30s")
		time.Sleep(30 * time.Second)
		info = tt.getStopInfo()
	}
	geoCoordinates := fmt.Sprintf("%f,%f", info.Lat, info.Lon)

	fetchWeather(apiKey, geoCoordinates, weather)
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for range tick.C {
		fetchWeather(apiKey, geoCoordinates, weather)
	}
}

func main() {
	stop := flag.Int("stop", 74640, "bus stop code")
	stop2 := flag.Int("stop2", 77484, "secondary bus stop code (touch screen toggles between the two)")
	touchDev := flag.String("touch", "", "touch input device path (auto-detected if empty)")
	debounce := flag.Duration("debounce", 100*time.Millisecond, "minimum interval between touch-triggered stop switches")
	rotate := flag.Bool("rotate", true, "rotate display 180 degrees")
	apiKey := flag.String("weather-key", "dd719ea57f1d4d44be6151200251209", "weatherapi.com API key")
	flag.Parse()

	tt1 := newTimeTable(*stop)
	tt1.start()

	var active atomic.Pointer[timeTable]
	active.Store(tt1)

	notify := make(chan struct{}, 1)
	var flip func()
	if *stop2 != 0 {
		tt2 := newTimeTable(*stop2)
		tt2.start()
		go watchTouch(*touchDev, tt1, tt2, &active, notify, *debounce)
		flip = func() { switchStop(tt1, tt2, &active, notify) }
	}

	var weather atomic.Pointer[string]
	weather.Store(new("loading..."))

	go weatherLoop(*apiKey, tt1, &weather)

	err := runDisplay(&active, &weather, *rotate, notify, flip)
	if err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}
