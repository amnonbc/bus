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

// advance cycles the display to the next screen, wrapping after the last.
// A nil screen is the clock.
func advance(screens []*timeTable, idx *atomic.Int32, active *atomic.Pointer[timeTable], notify chan<- struct{}) {
	next := (idx.Load() + 1) % int32(len(screens))
	idx.Store(next)
	tt := screens[next]
	active.Store(tt)
	logScreen(tt)
	select {
	case notify <- struct{}{}:
	default:
	}
}

func logScreen(tt *timeTable) {
	switch {
	case tt == nil:
		slog.Info("switched to clock")
	case tt.statusFrom != nil:
		slog.Info("switched to status messages")
	default:
		info := tt.getStopInfo()
		slog.Info("switched bus stop", "stop", info.Name, "towards", info.Towards)
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
	slog.Info("weather: waiting for coordinates to be set")
	<-tt.ready

	info := tt.getStopInfo()

	geoCoordinates := fmt.Sprintf("%f,%f", info.Lat, info.Lon)
	slog.Info("weather", "location", geoCoordinates)

	fetchWeather(apiKey, geoCoordinates, weather)
	tick := time.NewTicker(time.Hour)
	defer tick.Stop()
	for range tick.C {
		fetchWeather(apiKey, geoCoordinates, weather)
	}
}

func main() {
	var stops stopList
	flag.Var(&stops, "stop", "bus stop code, optionally with a background color, e.g. 12345:darkblue; repeat for multiple stops (touch cycles through stops, status messages and clock)")
	touchDev := flag.String("touch", "", "touch input device path (auto-detected if empty)")
	debounce := flag.Duration("debounce", 100*time.Millisecond, "minimum interval between touch-triggered stop switches")
	rotate := flag.Bool("rotate", true, "rotate display 180 degrees")
	debug := flag.Bool("debug", false, "log DRM device information and other diagnostic output")
	forceFB := flag.Bool("fb", false, "force framebuffer rendering, skipping DRM even if available")
	invert := flag.Bool("white", false, "white background: render black text on white instead of white on black")
	fontPath := flag.String("font", "", "path to TTF font file for bus numbers and times (default: Go Bold)")
	fontHeight := flag.Int("points", 120, "font height in points for bus numbers and times")
	textColor := flag.String("color", "", "text color as X11 color name (e.g. white, orange, darkred, cornflowerblue; default: white)")
	async := flag.Bool("async", false, "enable async display flips in pidisp")
	apiKey := flag.String("weather-key", "dd719ea57f1d4d44be6151200251209", "weatherapi.com API key")
	flag.Parse()

	if len(stops) == 0 {
		fmt.Fprintln(os.Stderr, "error: at least one -stop flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if *debug {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	tts := make([]*timeTable, len(stops))
	for i, s := range stops {
		tts[i] = newTimeTable(s.code)
		tts[i].bg = s.bg
		tts[i].start()
	}

	var active atomic.Pointer[timeTable]
	active.Store(tts[0])

	var idx atomic.Int32
	notify := make(chan struct{}, 1)
	// Screen order: each stop, then status messages, then the clock (nil).
	screens := append(tts, newStatusScreen(tts), nil)
	flip := func() { advance(screens, &idx, &active, notify) }
	go watchTouch(*touchDev, flip, *debounce)

	var weather atomic.Pointer[string]
	weather.Store(new("loading..."))

	go weatherLoop(*apiKey, tts[0], &weather)

	err := runDisplay(&active, &weather, fbOptions{
		rotate:     *rotate,
		debug:      *debug,
		forceFB:    *forceFB,
		invert:     *invert,
		fontPath:   *fontPath,
		fontHeight: *fontHeight,
		textColor:  *textColor,
		async:      *async,
	}, notify, flip)
	if err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}
