package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"
)

const tflBase = "https://countdown.api.tfl.gov.uk/interfaces/ura/instant_V1"

type resp []any

type Bus struct {
	Number string
	ETA    time.Time
}

func GetCountdownData(baseUrl string, stop int) ([]Bus, error) {
	buses := make([]Bus, 0, 3)
	u, err := url.Parse(baseUrl)
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

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		fmt.Println(r.Status)
		return nil, fmt.Errorf("bad status %s", r.Status)
	}
	dec := json.NewDecoder(r.Body)
	for {
		var b resp
		err := dec.Decode(&b)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println(err)
			return nil, err
		}
		if b[0].(float64) != 1 {
			continue
		}
		tnum, ok := b[3].(float64)
		if !ok {
			continue
		}
		t := int64(tnum)
		tm := time.UnixMilli(t)
		buses = append(buses, Bus{b[2].(string), tm})
	}
	slices.SortFunc(buses, func(a, b Bus) int {
		return a.ETA.Compare(b.ETA)
	})
	return buses, err
}
