package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

type Weather struct {
	Location struct {
		Name           string  `json:"name"`
		Region         string  `json:"region"`
		Country        string  `json:"country"`
		Lat            float64 `json:"lat"`
		Lon            float64 `json:"lon"`
		TzID           string  `json:"tz_id"`
		LocaltimeEpoch int     `json:"localtime_epoch"`
		Localtime      string  `json:"localtime"`
	} `json:"location"`
	Current struct {
		LastUpdatedEpoch int     `json:"last_updated_epoch"`
		LastUpdated      string  `json:"last_updated"`
		TempC            float64 `json:"temp_c"`
		TempF            float64 `json:"temp_f"`
		IsDay            int     `json:"is_day"`
		Condition        struct {
			Text string `json:"text"`
			Icon string `json:"icon"`
			Code int    `json:"code"`
		} `json:"condition"`
		WindMph    float64 `json:"wind_mph"`
		WindKph    float64 `json:"wind_kph"`
		WindDegree float64 `json:"wind_degree"`
		WindDir    string  `json:"wind_dir"`
		PressureMb float64 `json:"pressure_mb"`
		PressureIn float64 `json:"pressure_in"`
		PrecipMm   float64 `json:"precip_mm"`
		PrecipIn   float64 `json:"precip_in"`
		Humidity   float64 `json:"humidity"`
		Cloud      float64 `json:"cloud"`
		FeelslikeC float64 `json:"feelslike_c"`
		FeelslikeF float64 `json:"feelslike_f"`
		WindchillC float64 `json:"windchill_c"`
		WindchillF float64 `json:"windchill_f"`
		HeatindexC float64 `json:"heatindex_c"`
		HeatindexF float64 `json:"heatindex_f"`
		DewpointC  float64 `json:"dewpoint_c"`
		DewpointF  float64 `json:"dewpoint_f"`
		VisKm      float64 `json:"vis_km"`
		VisMiles   float64 `json:"vis_miles"`
		Uv         float64 `json:"uv"`
		GustMph    float64 `json:"gust_mph"`
		GustKph    float64 `json:"gust_kph"`
		ShortRad   float64 `json:"short_rad"`
		DiffRad    float64 `json:"diff_rad"`
		Dni        float64 `json:"dni"`
		Gti        float64 `json:"gti"`
	} `json:"current"`
}
type ErrResponse struct {
	Code      string `json:"Code"`
	Message   string `json:"Message"`
	Reference string `json:"Reference"`
}

var (
	conditionURL = "https://api.weatherapi.com/v1/current.json"
	API_KEY      = "dd719ea57f1d4d44be6151200251209"
	postcode     = "N2"
)

func (w Weather) String() string {
	c := w.Current
	return fmt.Sprintf("%s %v℃", c.Condition.Text, c.TempC)
}

func GetWeather() (Weather, error) {
	var w Weather
	uu, err := url.Parse(conditionURL)
	if err != nil {
		panic(err)
	}
	args := uu.Query()
	args.Add("key", API_KEY)
	args.Add("q", postcode)
	uu.RawQuery = args.Encode()

	r, err := http.Get(uu.String())
	if err != nil {
		log.Println(err)

		return w, err
	}
	defer r.Body.Close()
	log.Println("weather update returned", r.Status)
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
