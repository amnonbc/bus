// input_linux.go watches a touchscreen input device and toggles the active
// bus stop on each tap.
package main

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

// openTouchDevice blocks until the touch device at dev (or an auto-detected
// device if dev is empty) exists and is accessible, then returns an open file.
// It uses inotify on /dev/input to avoid polling.
func openTouchDevice(dev string) (*os.File, error) {
	ifd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("inotify init: %w", err)
	}
	defer syscall.Close(ifd)
	_, err = syscall.InotifyAddWatch(ifd, "/dev/input", syscall.IN_CREATE|syscall.IN_ATTRIB)
	if err != nil {
		return nil, fmt.Errorf("inotify watch /dev/input: %w", err)
	}

	buf := make([]byte, 4096)
	for {
		if dev == "" {
			dev = findTouchDevice()
		}
		if dev != "" {
			f, err := os.Open(dev)
			if err == nil {
				slog.Info("watching touch device", "dev", dev)
				return f, nil
			}
			if os.IsPermission(err) {
				// Node exists but udev has not yet applied group permissions;
				// watch the file itself so we wake as soon as IN_ATTRIB fires.
				syscall.InotifyAddWatch(ifd, dev, syscall.IN_ATTRIB)
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		}
		slog.Info("waiting for touch device", "dev", dev)
		_, err = syscall.Read(ifd, buf)
		if err != nil {
			return nil, fmt.Errorf("inotify read: %w", err)
		}
	}
}


// watchTouch reads touch events and calls flip on each finger-down
// (BTN_TOUCH value 1) event, subject to the debounce interval.
func watchTouch(dev string, flip func(), debounce time.Duration) {
	f, err := openTouchDevice(dev)
	if err != nil {
		slog.Error("open touch device", "err", err)
		return
	}
	defer f.Close()

	// Grab the device exclusively so the kernel doesn't also feed events to
	// /dev/mice, which would cause the DRM hardware cursor to appear on screen.
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), eviocgrab, 1); errno != 0 {
		slog.Warn("EVIOCGRAB", "dev", f.Name(), "err", errno)
	}

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
			flip()
		}
	}
}
