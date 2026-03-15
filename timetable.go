package main

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

type timeTable struct {
	stopID int
	busses atomic.Pointer[[]Bus]
}

func newTimeTable(stopID int) *timeTable {
	return &timeTable{stopID: stopID}
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
	b, err := GetCountdownData(tflBase, t.stopID)
	if err != nil {
		slog.Error("download timetable", "err", err)
		return
	}
	t.busses.Store(&b)
}

func (t *timeTable) downloadLoop() {
	tick := time.NewTicker(30 * time.Second)
	for range tick.C {
		t.download()
	}
}
