// weather.go fetches current conditions from the weatherapi.com API.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
)

type Weather struct {
	Current struct {
		TempC     float64 `json:"temp_c"`
		Condition struct {
			Text string `json:"text"`
		} `json:"condition"`
	} `json:"current"`
}

type ErrResponse struct {
	Message string `json:"Message"`
}

func (w Weather) String() string {
	c := w.Current
	return fmt.Sprintf("%s %.1f°C", c.Condition.Text, c.TempC)
}

func GetWeather(baseURL, apiKey, location string) (Weather, error) {
	var w Weather
	uu, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	args := uu.Query()
	args.Add("key", apiKey)
	args.Add("q", location)
	uu.RawQuery = args.Encode()

	r, err := http.Get(uu.String())
	if err != nil {
		slog.Error("weather request", "err", err)
		return w, err
	}
	defer r.Body.Close()
	slog.Info("weather update", "status", r.Status)
	if r.StatusCode != 200 {
		var resp ErrResponse
		err = json.NewDecoder(r.Body).Decode(&resp)
		if err != nil {
			return w, err
		}
		return w, errors.New(resp.Message)
	}
	err = json.NewDecoder(r.Body).Decode(&w)
	return w, err
}
