//go:build darwin

package main

import (
	"sync/atomic"
	"time"
)

func watchTouch(dev string, tt1, tt2 *timeTable, active *atomic.Pointer[timeTable], notify chan<- struct{}, debounce time.Duration) {
	// No touch input on macOS preview.
}