package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type StopInfo struct {
	Name    string
	Towards string
}

// GetStopInfoFromURA fetches stop name and direction from the TFL URA countdown
// API by requesting ReturnList=StopPointName,LineName,EstimatedTime,Towards and
// reading the first type 1 message. Format: [1, StopPointName, Towards, LineName, EstimatedTime]
func GetStopInfoFromURA(baseURL string, stopCode int) (StopInfo, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	args := u.Query()
	args.Add("StopCode1", strconv.Itoa(stopCode))
	args.Add("ReturnList", "StopPointName,LineName,EstimatedTime,Towards")
	u.RawQuery = args.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("User-Agent", "amnon_bus_times/2.0 (amnonbc@gmail.com)")
	r, err := httpClient.Do(req)
	if err != nil {
		return StopInfo{}, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return StopInfo{}, fmt.Errorf("bad status %s", r.Status)
	}

	dec := json.NewDecoder(r.Body)
	for {
		var b []any
		err := dec.Decode(&b)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return StopInfo{}, err
		}
		msgType, ok := b[0].(float64)
		if !ok {
			continue
		}
		if msgType != 1 || len(b) < 5 {
			continue
		}
		name, ok1 := b[1].(string)
		towards, ok2 := b[2].(string)
		if !ok1 || !ok2 {
			return StopInfo{}, fmt.Errorf("unexpected stop info format: %v", b)
		}
		return StopInfo{Name: name, Towards: towards}, nil
	}
	return StopInfo{}, fmt.Errorf("no stop info found for stop %d", stopCode)
}
