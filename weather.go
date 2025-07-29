package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

type Weather struct {
	LocalObservationDateTime time.Time `json:"LocalObservationDateTime"`
	EpochTime                int       `json:"EpochTime"`
	WeatherText              string    `json:"WeatherText"`
	WeatherIcon              int       `json:"WeatherIcon"`
	HasPrecipitation         bool      `json:"HasPrecipitation"`
	PrecipitationType        string    `json:"PrecipitationType"`
	IsDayTime                bool      `json:"IsDayTime"`
	Temperature              struct {
		Metric struct {
			Value    float64 `json:"Value"`
			Unit     string  `json:"Unit"`
			UnitType int     `json:"UnitType"`
		} `json:"Metric"`
		Imperial struct {
			Value    float64 `json:"Value"`
			Unit     string  `json:"Unit"`
			UnitType int     `json:"UnitType"`
		} `json:"Imperial"`
	} `json:"Temperature"`
	MobileLink string `json:"MobileLink"`
	Link       string `json:"Link"`
}

type ErrResponse struct {
	Code      string `json:"Code"`
	Message   string `json:"Message"`
	Reference string `json:"Reference"`
}

var (
	API_KEY     = "16cnkfx4543vJI1mMnV7RmXAmYAnQyrT"
	locationKey string
	postcode    = "N2 9LU"
)

func (w Weather) String() string {
	return fmt.Sprintf("%s %v℃", w.WeatherText, w.Temperature.Metric.Value)
}

func GetWeather() ([]Weather, error) {
	if locationKey == "" {
		var err error
		locationKey, err = GetLocationCode(postcode)
		if err != nil {
			return nil, err
		}
	}
	var w []Weather
	u := "https://dataservice.accuweather.com/currentconditions/v1/" + locationKey
	uu, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	args := uu.Query()
	args.Add("apikey", API_KEY)
	uu.RawQuery = args.Encode()

	r, err := http.Get(uu.String())
	if err != nil {
		log.Println(err)

		return nil, err
	}
	defer r.Body.Close()
	left := r.Header.Get("RateLimit-Remaining")
	log.Println("weather update returned", r.Status, "remaining", left)
	if r.StatusCode != 200 {
		var resp ErrResponse
		err = json.NewDecoder(r.Body).Decode(&resp)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(resp.Message)
	}
	err = json.NewDecoder(r.Body).Decode(&w)
	return w, err
}

type locResp struct {
	Key string
}

func GetLocationCode(postcode string) (string, error) {
	var w []locResp
	u := "http://dataservice.accuweather.com/locations/v1/postalcodes/search"
	uu, err := url.Parse(u)
	if err != nil {
		panic(err)
	}
	args := uu.Query()
	args.Add("apikey", API_KEY)
	args.Add("q", postcode)
	uu.RawQuery = args.Encode()

	r, err := http.Get(uu.String())
	if err != nil {
		log.Println(err)

		return "", err
	}
	defer r.Body.Close()
	left := r.Header.Get("RateLimit-Remaining")
	log.Println("GetLocationCode returned", r.Status, "remaining", left)
	if r.StatusCode != 200 {
		var resp ErrResponse
		err = json.NewDecoder(r.Body).Decode(&resp)
		if err != nil {
			return "", err
		}
		return "", errors.New(resp.Message)
	}
	err = json.NewDecoder(r.Body).Decode(&w)
	if err != nil {
		return "", err
	}
	if len(w) == 0 {
		return "", errors.New("no results")
	}
	return w[0].Key, nil
}
