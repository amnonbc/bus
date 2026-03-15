package main

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type timeTable struct {
	sync.Mutex
	stopID int
	busses []Bus
}

func newTimeTable(stopID int) *timeTable {
	return &timeTable{stopID: stopID}
}

func (t *timeTable) start() {
	t.download()
	go t.downloadLoop()
}

func (t *timeTable) getBuses() []Bus {
	t.Lock()
	defer t.Unlock()
	result := make([]Bus, len(t.busses))
	copy(result, t.busses)
	return result
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
	t.Lock()
	t.busses = b
	t.Unlock()
}

func (t *timeTable) downloadLoop() {
	tick := time.NewTicker(30 * time.Second)
	for range tick.C {
		t.download()
	}
}
