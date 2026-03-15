package main

import (
	"flag"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

const weatherURL = "https://api.weatherapi.com/v1/current.json"

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

func main() {
	stop := flag.Int("stop", 74640, "bus stop code")
	rotate := flag.Bool("rotate", true, "rotate display 180 degrees")
	apiKey := flag.String("weather-key", "dd719ea57f1d4d44be6151200251209", "weatherapi.com API key")
	location := flag.String("location", "N2", "location for weather (postcode or city)")
	flag.Parse()

	tt := newTimeTable(*stop)
	tt.start()

	var weather atomic.Pointer[string]
	weather.Store(new("loading..."))

	go func() {
		fetchWeather(*apiKey, *location, &weather)
		tick := time.NewTicker(time.Hour)
		defer tick.Stop()
		for range tick.C {
			fetchWeather(*apiKey, *location, &weather)
		}
	}()

	if err := runDisplay(tt, &weather, *rotate); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}
