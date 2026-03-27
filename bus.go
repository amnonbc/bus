// bus.go fetches live bus arrival times from the TFL URA countdown API.
// API documentation: https://content.tfl.gov.uk/tfl-live-bus-river-bus-arrivals-api-documentation.pdf
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

var httpClient = &http.Client{Timeout: 30 * time.Second}

const tflBase = "https://countdown.api.tfl.gov.uk/interfaces/ura/instant_V1"

type Bus struct {
	Number string
	ETA    time.Time
}

type StopInfo struct {
	Name    string
	Towards string
	Lat     float64
	Lon     float64
}

// uraMessage is a decoded URA type-1 row:
//
//	[1, StopPointName, Towards, Latitude, Longitude, LineName, EstimatedTime_ms]
type uraMessage struct {
	Stop StopInfo
	Bus  Bus
}

func (m *uraMessage) UnmarshalJSON(data []byte) error {
	var arr [7]json.RawMessage
	err := json.Unmarshal(data, &arr)
	if err != nil {
		return err
	}
	var msgType int
	err = json.Unmarshal(arr[0], &msgType)
	if err != nil {
		return fmt.Errorf("unmarshal message type: %w", err)
	}
	if msgType != 1 {
		return fmt.Errorf("not a type-1 message %s", data)
	}
	var etaMS int64
	err = errors.Join(
		json.Unmarshal(arr[1], &m.Stop.Name),
		json.Unmarshal(arr[2], &m.Stop.Towards),
		json.Unmarshal(arr[3], &m.Stop.Lat),
		json.Unmarshal(arr[4], &m.Stop.Lon),
		json.Unmarshal(arr[5], &m.Bus.Number),
		json.Unmarshal(arr[6], &etaMS),
	)
	if err != nil {
		return err
	}
	m.Bus.ETA = time.UnixMilli(etaMS)
	return nil
}

// GetBusData fetches arrivals and stop metadata for the given stop in a single
// request. The URA API returns type-1 messages in the form:
//
//	[1, StopPointName, Towards, Latitude, Longitude, LineName, EstimatedTime_ms]
func GetBusData(baseURL string, stop int) ([]Bus, StopInfo, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	q := u.Query()
	q.Set("StopCode1", strconv.Itoa(stop))
	q.Set("ReturnList", "StopPointName,LineName,EstimatedTime,Towards,Latitude,Longitude")
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
		var raw json.RawMessage
		err := dec.Decode(&raw)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			slog.Error("decode TFL response", "err", err)
			return nil, StopInfo{}, err
		}
		var msg uraMessage
		err = json.Unmarshal(raw, &msg)
		if err != nil {
			slog.Debug("decode TFL response", "err", err)
			continue // non-type-1 or malformed row
		}
		if info.Name == "" {
			info = msg.Stop
		}
		buses = append(buses, msg.Bus)
	}
	slices.SortFunc(buses, func(a, b Bus) int {
		return a.ETA.Compare(b.ETA)
	})
	return buses, info, nil
}
