// input_linux.go watches a touchscreen input device and toggles the active
// bus stop on each tap.
package main

import (
	"encoding/binary"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	evKey     = 0x01
	btnTouch  = 0x14a
	eviocgrab = 0x40044590 // EVIOCGRAB: exclusively grab the device
)

// inputEvent mirrors struct input_event from <linux/input.h> for 32-bit ARM.
// On 32-bit Linux timeval uses two uint32s, giving a 16-byte struct total.
type inputEvent struct {
	Sec   uint32
	Usec  uint32
	Type  uint16
	Code  uint16
	Value int32
}

// findTouchDevice scans /sys/class/input for a known touchscreen driver name.
func findTouchDevice() string {
	names, _ := filepath.Glob("/sys/class/input/event*/device/name")
	for _, p := range names {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		lower := strings.ToLower(string(data))
		if strings.Contains(lower, "ft5") || strings.Contains(lower, "touch") {
			// p is .../event0/device/name — event dir is three levels up
			event := filepath.Base(filepath.Dir(filepath.Dir(p)))
			return "/dev/input/" + event
		}
	}
	return ""
}

// watchTouch reads touch events and toggles the active timetable between tt1
// and tt2 on each finger-down (BTN_TOUCH value 1) event. It sends on notify
// after each switch so the display can redraw immediately.
func watchTouch(dev string, tt1, tt2 *timeTable, active *atomic.Pointer[timeTable], notify chan<- struct{}, debounce time.Duration) {
	if dev == "" {
		dev = findTouchDevice()
	}
	if dev == "" {
		slog.Warn("no touch device found; touch switching disabled")
		return
	}

	f, err := os.Open(dev)
	if err != nil {
		slog.Error("open touch device", "dev", dev, "err", err)
		return
	}
	defer f.Close()

	// Grab the device exclusively so the kernel doesn't also feed events to
	// /dev/mice, which would cause the DRM hardware cursor to appear on screen.
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), eviocgrab, 1); errno != 0 {
		slog.Warn("EVIOCGRAB", "dev", dev, "err", errno)
	}
	slog.Info("watching touch device", "dev", dev)

	var ev inputEvent
	var lastSwitch time.Time
	for {
		err := binary.Read(f, binary.LittleEndian, &ev)
		if err != nil {
			slog.Error("read touch event", "err", err)
			return
		}
		if ev.Type == evKey && ev.Code == btnTouch && ev.Value == 1 {
			if time.Since(lastSwitch) < debounce {
				continue
			}
			lastSwitch = time.Now()
			switchStop(tt1, tt2, active, notify)
		}
	}
}
