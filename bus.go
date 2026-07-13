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

// uraMessage is a decoded URA type-1 row:
//
//	[1, StopPointName, Towards, StopPointState, Latitude, Longitude, LineName, EstimatedTime_ms]
type uraMessage struct {
	Stop  StopInfo
	Bus   Bus
	State int
}

func (m *uraMessage) UnmarshalJSON(data []byte) error {
	var arr []json.RawMessage
	err := json.Unmarshal(data, &arr)
	if err != nil {
		return err
	}
	if len(arr) < 7 {
		return fmt.Errorf("type-1 message too short")
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
		json.Unmarshal(arr[3], &m.State),
		json.Unmarshal(arr[4], &m.Stop.Lat),
		json.Unmarshal(arr[5], &m.Stop.Lon),
		json.Unmarshal(arr[6], &m.Bus.Number),
		json.Unmarshal(arr[7], &etaMS),
	)
	if err != nil {
		return err
	}

	m.Bus.ETA = time.UnixMilli(etaMS)
	return nil
}

// logClockSkew parses a URA type-4 message [4, URAVersion, TimeStamp_ms] and
// logs the difference between TFL's server clock and the local machine clock
// if the difference is more than 10 seconds.
func logClockSkew(raw json.RawMessage) {
	var arr [3]json.RawMessage
	err := json.Unmarshal(raw, &arr)
	if err != nil {
		return
	}
	var tsMS int64
	err = json.Unmarshal(arr[2], &tsMS)
	if err != nil {
		return
	}
	skew := time.UnixMilli(tsMS).Sub(time.Now())
	metricTFLClockSkew.Set(skew.Seconds())
	if skew.Abs() > 10*time.Second {
		slog.Warn("TFL clock skew", "skew", skew)
		return
	}
	slog.Debug("TFL clock skew", "skew", skew)
}

// parseStopArray parses a URA type-0 message [0, StopPointName, StopPointState]
func parseStopArray(raw json.RawMessage) (StopInfo, error) {
	var arr []json.RawMessage
	err := json.Unmarshal(raw, &arr)
	if err != nil {
		return StopInfo{}, err
	}
	if len(arr) < 3 {
		return StopInfo{}, fmt.Errorf("type-0 message too short")
	}
	var msgType int
	err = json.Unmarshal(arr[0], &msgType)
	if err != nil || msgType != 0 {
		return StopInfo{}, fmt.Errorf("not a type-0 message")
	}

	var info StopInfo
	err = errors.Join(
		json.Unmarshal(arr[1], &info.Name),
		json.Unmarshal(arr[2], &info.State),
	)
	return info, err
}

// parseStatusMessage parses a URA type-2 message with various possible formats
func parseStatusMessage(raw json.RawMessage) (StatusMessage, error) {
	var arr []json.RawMessage
	err := json.Unmarshal(raw, &arr)
	if err != nil {
		return StatusMessage{}, err
	}
	if len(arr) < 5 {
		return StatusMessage{}, fmt.Errorf("type-2 message too short")
	}
	var msgType int
	err = json.Unmarshal(arr[0], &msgType)
	if err != nil || msgType != 2 {
		return StatusMessage{}, fmt.Errorf("not a type-2 message")
	}

	var msg StatusMessage
	msg.Priority = 1

	if len(arr) == 8 {
		// Format with ReturnList fields: [2, StopName, Direction, StopState, Lat, Lon, MessageType, MessageText]
		err = errors.Join(
			json.Unmarshal(arr[6], &msg.Type),
			json.Unmarshal(arr[7], &msg.Text),
		)
		return msg, err
	}

	if len(arr) >= 5 && len(arr) < 8 {
		// Format: [2, StopName, StopState, MessageType, MessageText]
		err = errors.Join(
			json.Unmarshal(arr[3], &msg.Type),
			json.Unmarshal(arr[4], &msg.Text),
		)
		return msg, err
	}

	if len(arr) >= 7 {
		// Format: [2, MessageUUID, MessageText, MessageType, MessagePriority, StartTime_ms, ExpireTime_ms]
		var startMS, expireMS int64
		err = errors.Join(
			json.Unmarshal(arr[2], &msg.Text),
			json.Unmarshal(arr[3], &msg.Type),
			json.Unmarshal(arr[4], &msg.Priority),
			json.Unmarshal(arr[5], &startMS),
			json.Unmarshal(arr[6], &expireMS),
		)
		if err == nil {
			msg.Start = time.UnixMilli(startMS)
			msg.Expire = time.UnixMilli(expireMS)
		}
		return msg, err
	}

	return StatusMessage{}, fmt.Errorf("unknown type-2 format")
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
	req.Header.Set("User-Agent", "amnon_bus_times/2.0 (amnonbc@gmail.com)")

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
		var arr []json.RawMessage
		var msgType int
		json.Unmarshal(raw, &arr)
		if len(arr) > 0 {
			json.Unmarshal(arr[0], &msgType)
		}

		switch msgType {
		case 0:
			stopInfo, err := parseStopArray(raw)
			if err != nil {
				slog.Warn("decode TFL stop array", "err", err)
				continue
			}
			if info.Name == "" {
				info = stopInfo
			} else {
				info.State = stopInfo.State
			}
		case 1:
			var msg uraMessage
			err := json.Unmarshal(raw, &msg)
			if err != nil {
				slog.Warn("decode TFL response", "err", err)
				continue
			}
			if info.Name == "" {
				info = msg.Stop
				info.State = msg.State
			}
			buses = append(buses, msg.Bus)
		case 2:
			msg, err := parseStatusMessage(raw)
			if err != nil {
				slog.Warn("decode TFL status message", "err", err)
				continue
			}
			info.Status = append(info.Status, msg)

			if info.Name == "" && len(arr) > 1 {
				var stopName string
				if err := json.Unmarshal(arr[1], &stopName); err == nil {
					info.Name = stopName
				}
			}
		case 4:
			logClockSkew(raw)
		default:
			slog.Warn("unknown message type", "type", msgType)
		}
	}
	slices.SortFunc(buses, func(a, b Bus) int {
		return a.ETA.Compare(b.ETA)
	})
	metricTFLBuses.Set(float64(len(buses)))
	return buses, info, nil
}
