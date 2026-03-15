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

func GetCountdownData(baseURL string, stop int) ([]Bus, error) {
	buses := make([]Bus, 0, 3)
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	args := u.Query()
	args.Add("StopCode1", strconv.Itoa(stop))
	u.RawQuery = args.Encode()
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("User-Agent", "amnon_bus_times/2.0 (amnonbc@gmail.com)")

	r, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("bad status %s", r.Status)
	}
	dec := json.NewDecoder(r.Body)
	for {
		var b []any
		err := dec.Decode(&b)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			slog.Error("decode TFL response", "err", err)
			return nil, err
		}
		if len(b) < 4 {
			continue
		}
		msgType, ok := b[0].(float64)
		if !ok || msgType != 1 {
			continue
		}
		number, ok := b[2].(string)
		if !ok {
			continue
		}
		tnum, ok := b[3].(float64)
		if !ok {
			continue
		}
		buses = append(buses, Bus{number, time.UnixMilli(int64(tnum))})
	}
	slices.SortFunc(buses, func(a, b Bus) int {
		return a.ETA.Compare(b.ETA)
	})
	return buses, nil
}
