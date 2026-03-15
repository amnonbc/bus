package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const testAPIKey = "dd719ea57f1d4d44be6151200251209"

func TestGetWeather(t *testing.T) {
	w, err := GetWeather(weatherURL, testAPIKey, "N2")
	require.NoError(t, err)
	require.NotEmpty(t, w)
	t.Log(w.String())
}

func TestGetWeatherMock(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "testkey", r.URL.Query().Get("key"))
		require.Equal(t, "N1", r.URL.Query().Get("q"))
		fmt.Fprintln(w, `{"current":{"temp_c":15.5,"condition":{"text":"Partly cloudy"}}}`)
	}))
	defer ts.Close()

	w, err := GetWeather(ts.URL, "testkey", "N1")
	require.NoError(t, err)
	require.Equal(t, "Partly cloudy 15.5°C", w.String())
}

func TestGetWeatherAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintln(w, `{"Message":"API key invalid"}`)
	}))
	defer ts.Close()

	_, err := GetWeather(ts.URL, "badkey", "N1")
	require.ErrorContains(t, err, "API key invalid")
}

func TestGetWeatherNetworkError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close()

	_, err := GetWeather(ts.URL, "key", "N1")
	require.Error(t, err)
}
