package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// tbHandler routes slog output through t.Log so it is shown only on failure.
type tbHandler struct{ tb testing.TB }

func (h tbHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h tbHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h tbHandler) WithGroup(string) slog.Handler            { return h }
func (h tbHandler) Handle(_ context.Context, r slog.Record) error {
	h.tb.Log(r.Message)
	return nil
}

func setTestLogger(tb testing.TB) {
	slog.SetDefault(slog.New(tbHandler{tb}))
}

var testData = `
[4,"1.0",1701512836819]
[1,"Midhurst Avenue","East Finchley",51.5921,-0.1780,"102",1701512991000]
[1,"Midhurst Avenue","East Finchley",51.5921,-0.1780,"102",1701513626000]
[1,"Midhurst Avenue","East Finchley",51.5921,-0.1780,"234",1701513209000]
[1,"Midhurst Avenue","East Finchley",51.5921,-0.1780,"102",1701514468000]
[1,"Midhurst Avenue","East Finchley",51.5921,-0.1780,"102",1701514536000]
[1,"Midhurst Avenue","East Finchley",51.5921,-0.1780,"234",1701514596000]
[1,"Midhurst Avenue","East Finchley",51.5921,-0.1780,"234",1701513860000]
`

func TestGetBusData(t *testing.T) {
	setTestLogger(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, testData)
	}))
	defer ts.Close()

	buses, info, err := GetBusData(ts.URL, 123)
	require.NoError(t, err)

	require.Equal(t, 7, len(buses))
	require.Equal(t, "102", buses[0].Number)
	require.Equal(t, time.Date(2023, time.December, 2, 10, 29, 51, 0, time.Local), buses[0].ETA)
	require.Equal(t, "Midhurst Avenue", info.Name)
	require.Equal(t, "East Finchley", info.Towards)
	require.InDelta(t, 51.5921, info.Lat, 0.0001)
	require.InDelta(t, -0.1780, info.Lon, 0.0001)
}

func TestGetBusDataErr(t *testing.T) {
	setTestLogger(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer ts.Close()

	buses, _, err := GetBusData(ts.URL, 123)
	require.Error(t, err)
	require.Nil(t, buses)
}

func TestGetBusDataNetwork(t *testing.T) {
	setTestLogger(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	ts.Close()

	buses, _, err := GetBusData(ts.URL, 123)
	require.Error(t, err)
	require.Nil(t, buses)
}
