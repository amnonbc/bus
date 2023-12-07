package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

func (w Weather) String() string {
	return fmt.Sprintf("%s %v%s", w.WeatherText, w.Temperature.Metric.Value, w.Temperature.Metric.Unit)
}

func GetWeather() (Weather, error) {
	var w []Weather
	u := "https://dataservice.accuweather.com/currentconditions/v1/49505_PC?apikey=16cnkfx4543vJI1mMnV7RmXAmYAnQyrT"

	r, err := http.Get(u)
	if err != nil {
		log.Println(err)
		return Weather{}, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return Weather{}, fmt.Errorf("Bad HTTP status %d %s", r.StatusCode, r.Status)
	}

	err = json.NewDecoder(r.Body).Decode(&w)
	return w[0], err
}
