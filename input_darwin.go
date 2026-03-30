//go:build darwin

package main

import "time"

func watchTouch(_ string, _ func(), _ time.Duration) {
	// No touch input on macOS preview.
}