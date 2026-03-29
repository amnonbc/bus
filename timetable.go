// timetable.go manages a periodically-refreshed list of bus arrivals for a
// single stop, safe for concurrent access via atomic pointers.
package main

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

type timeTable struct {
	stopID int
	ready  chan struct{}
	signal sync.Once
	info   atomic.Pointer[StopInfo]
	busses atomic.Pointer[[]Bus]
}

func newTimeTable(stopID int) *timeTable {
	return &timeTable{
		stopID: stopID,
		ready:  make(chan struct{}),
	}
}

func (t *timeTable) start() {
	t.download()
	go t.downloadLoop()
}

func (t *timeTable) getBuses() []Bus {
	if p := t.busses.Load(); p != nil {
		return *p
	}
	return nil
}

func (t *timeTable) getStopInfo() StopInfo {
	if p := t.info.Load(); p != nil {
		return *p
	}
	return StopInfo{}
}

type delay time.Duration

func (d delay) String() string {
	s := int(time.Duration(d).Seconds())
	m := s / 60
	s -= 60 * m
	return fmt.Sprintf("%d:%02d", m, s)
}

func fromTime(t time.Time) delay {
	return delay(time.Until(t).Round(time.Second))
}

func (t *timeTable) download() {
	buses, info, err := GetBusData(tflBase, t.stopID)
	if err != nil {
		slog.Error("download timetable", "err", err)
		return
	}
	t.busses.Store(&buses)
	t.info.Store(&info)
	t.signal.Do(func() { close(t.ready) })
}

func (t *timeTable) downloadLoop() {
	tick := time.NewTicker(30 * time.Second)
	for range tick.C {
		t.download()
	}
}
