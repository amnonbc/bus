//go:build darwin

package main

import "sync/atomic"

func watchTouch(dev string, tt1, tt2 *timeTable, active *atomic.Pointer[timeTable]) {
	// No touch input on macOS preview.
}