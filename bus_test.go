package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var testData = `
[4,"1.0",1701512836819]
[1,"Midhurst Avenue","102",1701512991000]
[1,"Midhurst Avenue","102",1701513626000]
[1,"Midhurst Avenue","234",1701513209000]
[1,"Midhurst Avenue","102",1701514468000]
[1,"Midhurst Avenue","102",1701514536000]
[1,"Midhurst Avenue","234",1701514596000]
[1,"Midhurst Avenue","234",1701513860000]
`

func TestGetCountdownData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, testData)
	}))
	defer ts.Close()

	got, err := GetCountdownData(ts.URL, 123)
	require.NoError(t, err)

	require.Equal(t, 7, len(got))
	require.Equal(t, "102", got[0].Number)
	require.Equal(t, time.Date(2023, time.December, 2, 10, 29, 51, 0, time.Local), got[0].ETA)
}

func TestGetCountdownDataErr(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	got, err := GetCountdownData(ts.URL, 123)
	require.Error(t, err)
	require.Nil(t, got)
}

func TestGetCountdownDataNetwork(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	ts.Close()

	got, err := GetCountdownData(ts.URL, 123)
	require.Error(t, err)
	require.Nil(t, got)
}
