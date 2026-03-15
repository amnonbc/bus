//go:build linux

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
			parts := strings.Split(p, "/")
			for _, part := range parts {
				if strings.HasPrefix(part, "event") {
					return "/dev/input/" + part
				}
			}
		}
	}
	return ""
}

// watchTouch reads touch events and toggles the active timetable between tt1
// and tt2 on each finger-down (BTN_TOUCH value 1) event.
func watchTouch(dev string, tt1, tt2 *timeTable, active *atomic.Pointer[timeTable]) {
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
		if err := binary.Read(f, binary.LittleEndian, &ev); err != nil {
			slog.Error("read touch event", "err", err)
			return
		}
		if ev.Type == evKey && ev.Code == btnTouch && ev.Value == 1 {
			if time.Since(lastSwitch) < time.Second {
				continue
			}
			lastSwitch = time.Now()
			if active.Load() == tt1 {
				active.Store(tt2)
			} else {
				active.Store(tt1)
			}
			current := active.Load()
			slog.Info("touch: switched bus stop", "stop", current.info.Name, "towards", current.info.Towards)
		}
	}
}
