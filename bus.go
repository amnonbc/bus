// bus.go fetches live bus arrival times from the TFL URA countdown API.
// API documentation: https://content.tfl.gov.uk/tfl-live-bus-river-bus-arrivals-api-documentation.pdf
package main

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
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

type StatusMessage struct {
	Text     string
	Type     int
	Priority int
	Start    time.Time
	Expire   time.Time
}

type StopInfo struct {
	Name    string
	Towards string
	Lat     float64
	Lon     float64
	State   int
	Status  []StatusMessage
}

// readFields decodes consecutive JSON values from dec into ptrs.
func readFields(dec *jsontext.Decoder, ptrs ...any) error {
	for _, p := range ptrs {
		err := json.UnmarshalDecode(dec, p)
		if err != nil {
			return err
		}
	}
	return nil
}

// readRest decodes all remaining elements of the current array.
func readRest(dec *jsontext.Decoder) ([]jsontext.Value, error) {
	var rest []jsontext.Value
	for dec.PeekKind() != ']' {
		var v jsontext.Value
		err := json.UnmarshalDecode(dec, &v)
		if err != nil {
			return nil, err
		}
		rest = append(rest, v)
	}
	return rest, nil
}

// readHeader consumes the opening '[' and returns the message type.
func readHeader(dec *jsontext.Decoder) (int, error) {
	tok, err := dec.ReadToken()
	if err != nil {
		return 0, err
	}
	if tok.Kind() != '[' {
		return 0, fmt.Errorf("message is not an array")
	}
	var msgType int
	err = json.UnmarshalDecode(dec, &msgType)
	return msgType, err
}

// readArrival decodes the rest of a type-1 message:
//
//	[1, StopPointName, Towards, StopPointState, Latitude, Longitude, LineName, EstimatedTime_ms]
func readArrival(dec *jsontext.Decoder) (StopInfo, Bus, error) {
	var stop StopInfo
	var bus Bus
	var etaMS int64
	err := readFields(dec, &stop.Name, &stop.Towards, &stop.State, &stop.Lat, &stop.Lon, &bus.Number, &etaMS)
	bus.ETA = time.UnixMilli(etaMS)
	return stop, bus, err
}

// readStop decodes the rest of a type-0 message: [0, StopPointName, StopPointState]
func readStop(dec *jsontext.Decoder) (StopInfo, error) {
	var info StopInfo
	err := readFields(dec, &info.Name, &info.State)
	return info, err
}

// readStatus decodes the rest of a type-2 message and returns the stop name it
// carries. Two layouts are seen in practice, distinguished by length:
//
//	[2, StopName, Direction, StopState, Lat, Lon, MessageType, MessageText]
//	[2, StopName, StopState, MessageType, MessageText]
func readStatus(dec *jsontext.Decoder) (msg StatusMessage, stopName string, err error) {
	rest, err := readRest(dec)
	if err != nil {
		return msg, "", err
	}
	var typeIdx int
	switch {
	case len(rest) == 7:
		typeIdx = 5
	case len(rest) >= 4 && len(rest) < 7:
		typeIdx = 2
	default:
		return msg, "", fmt.Errorf("unknown type-2 format with %d fields", len(rest)+1)
	}
	msg.Priority = 1
	err = errors.Join(
		json.Unmarshal(rest[0], &stopName),
		json.Unmarshal(rest[typeIdx], &msg.Type),
		json.Unmarshal(rest[typeIdx+1], &msg.Text),
	)
	return msg, stopName, err
}

// logClockSkew reads the rest of a type-4 message [4, URAVersion, TimeStamp_ms]
// and logs the difference between TFL's server clock and the local machine
// clock if the difference is more than 10 seconds.
func logClockSkew(dec *jsontext.Decoder) {
	var version string
	var tsMS int64
	err := readFields(dec, &version, &tsMS)
	if err != nil {
		return
	}
	skew := time.Until(time.UnixMilli(tsMS))
	metricTFLClockSkew.Set(skew.Seconds())
	if skew.Abs() > 10*time.Second {
		slog.Warn("TFL clock skew", "skew", skew)
		return
	}
	slog.Debug("TFL clock skew", "skew", skew)
}

// accumulator collects the decoded content of a URA response.
type accumulator struct {
	buses []Bus
	info  StopInfo
}

// add decodes one raw URA message and merges it into the accumulator.
// Malformed messages are logged and skipped.
func (a *accumulator) add(raw jsontext.Value) {
	dec := jsontext.NewDecoder(bytes.NewReader(raw))
	msgType, err := readHeader(dec)
	if err != nil {
		slog.Warn("decode TFL message header", "err", err)
		return
	}
	switch msgType {
	case 0:
		a.addStop(dec)
	case 1:
		a.addArrival(dec)
	case 2:
		a.addStatus(dec)
	case 4:
		logClockSkew(dec)
	default:
		slog.Warn("unknown message type", "type", msgType)
	}
}

func (a *accumulator) addStop(dec *jsontext.Decoder) {
	stop, err := readStop(dec)
	if err != nil {
		slog.Warn("decode TFL stop array", "err", err)
		return
	}
	if a.info.Name == "" {
		a.info = stop
		return
	}
	a.info.State = stop.State
}

func (a *accumulator) addArrival(dec *jsontext.Decoder) {
	stop, bus, err := readArrival(dec)
	if err != nil {
		slog.Warn("decode TFL response", "err", err)
		return
	}
	if a.info.Name == "" {
		a.info = stop
	}
	a.buses = append(a.buses, bus)
}

func (a *accumulator) addStatus(dec *jsontext.Decoder) {
	msg, stopName, err := readStatus(dec)
	if err != nil {
		slog.Warn("decode TFL status message", "err", err)
		return
	}
	a.info.Status = append(a.info.Status, msg)
	if a.info.Name == "" {
		a.info.Name = stopName
	}
}

// GetBusData fetches arrivals and stop metadata for the given stop in a single
// request. The URA API returns type-1 messages for arrivals, type-2 messages for
// status/disruptions, and type-4 messages for server timestamp.
func GetBusData(baseURL string, stop int) ([]Bus, StopInfo, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	q := u.Query()
	q.Set("StopCode1", strconv.Itoa(stop))
	q.Set("ReturnList", "StopPointName,LineName,EstimatedTime,Towards,Latitude,Longitude,StopPointState,MessageText,MessageType")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("User-Agent", "amnon_bus_times/2.0")

	r, err := httpClient.Do(req)
	if err != nil {
		metricTFLRequests.WithLabelValues("error").Inc()
		return nil, StopInfo{}, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		metricTFLRequests.WithLabelValues("error").Inc()
		return nil, StopInfo{}, fmt.Errorf("bad status %s", r.Status)
	}
	metricTFLRequests.WithLabelValues("ok").Inc()

	var acc accumulator
	dec := jsontext.NewDecoder(r.Body)
	for {
		raw, err := dec.ReadValue()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			slog.Error("decode TFL response", "err", err)
			return nil, StopInfo{}, err
		}
		acc.add(raw)
	}
	buses, info := acc.buses, acc.info
	slices.SortFunc(buses, func(a, b Bus) int {
		return a.ETA.Compare(b.ETA)
	})
	metricTFLBuses.Set(float64(len(buses)))
	return buses, info, nil
}
