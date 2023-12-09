package main

import (
	"encoding/json"
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

var (
	API_KEY  = "16cnkfx4543vJI1mMnV7RmXAmYAnQyrT"
	LOCATION = "49505_PC"
)

func (w Weather) String() string {
	return fmt.Sprintf("%s %v%s", w.WeatherText, w.Temperature.Metric.Value, w.Temperature.Metric.Unit)
}

func GetWeather() ([]Weather, error) {
	var w []Weather
	u := "https://dataservice.accuweather.com/currentconditions/v1/" + LOCATION
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
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("Bad HTTP status %d %s", r.StatusCode, r.Status)
	}

	err = json.NewDecoder(r.Body).Decode(&w)
	return w, err
}
