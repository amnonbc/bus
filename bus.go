// bus.go fetches live bus arrival times from the TFL URA countdown API.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

const tflBase = "https://countdown.api.tfl.gov.uk/interfaces/ura/instant_V1"

type Bus struct {
	Number string
	ETA    time.Time
}

type StopInfo struct {
	Name    string
	Towards string
}

// GetBusData fetches arrivals and stop metadata for the given stop in a single
// request. The URA API returns type-1 messages in the form:
//
//	[1, StopPointName, Towards, LineName, EstimatedTime_ms]
func GetBusData(baseURL string, stop int) ([]Bus, StopInfo, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	q := u.Query()
	q.Set("StopCode1", strconv.Itoa(stop))
	q.Set("ReturnList", "StopPointName,LineName,EstimatedTime,Towards")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("User-Agent", "amnon_bus_times/2.0 (amnonbc@gmail.com)")

	r, err := httpClient.Do(req)
	if err != nil {
		return nil, StopInfo{}, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, StopInfo{}, fmt.Errorf("bad status %s", r.Status)
	}

	var buses []Bus
	var info StopInfo
	dec := json.NewDecoder(r.Body)
	for {
		var msg []any
		err := dec.Decode(&msg)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			slog.Error("decode TFL response", "err", err)
			return nil, StopInfo{}, err
		}
		if len(msg) < 5 {
			continue
		}
		msgType, ok := msg[0].(float64)
		if !ok || msgType != 1 {
			continue
		}
		name, ok1 := msg[1].(string)
		towards, ok2 := msg[2].(string)
		number, ok3 := msg[3].(string)
		tnum, ok4 := msg[4].(float64)
		if !ok1 || !ok2 || !ok3 || !ok4 {
			continue
		}
		if info.Name == "" {
			info = StopInfo{Name: name, Towards: towards}
		}
		buses = append(buses, Bus{Number: number, ETA: time.UnixMilli(int64(tnum))})
	}
	slices.SortFunc(buses, func(a, b Bus) int {
		return a.ETA.Compare(b.ETA)
	})
	return buses, info, nil
}
