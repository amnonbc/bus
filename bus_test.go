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
[4,"1.0",1783956388076]
[1,"Midhurst Avenue","East Finchley",0,51.590935,-0.153627,"102",1783956406000]
[1,"Midhurst Avenue","East Finchley",0,51.590935,-0.153627,"102",1783956663000]
[1,"Midhurst Avenue","East Finchley",0,51.590935,-0.153627,"102",1783956678000]
[1,"Midhurst Avenue","East Finchley",0,51.590935,-0.153627,"234",1783957378000]
[1,"Midhurst Avenue","East Finchley",0,51.590935,-0.153627,"102",1783957282000]
[1,"Midhurst Avenue","East Finchley",0,51.590935,-0.153627,"102",1783957540000]
[1,"Midhurst Avenue","East Finchley",0,51.590935,-0.153627,"234",1783957877000]
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
	require.Equal(t, time.Date(2026, time.July, 13, 16, 26, 46, 0, time.Local), buses[0].ETA)
	require.Equal(t, "Midhurst Avenue", info.Name)
	require.Equal(t, "East Finchley", info.Towards)
	require.InDelta(t, 51.590935, info.Lat, 0.0001)
	require.InDelta(t, -0.153627, info.Lon, 0.0001)
	require.Equal(t, 0, info.State)
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

var testDataWithStatus = `
[4,"1.0",1783956388076]
[1,"Midhurst Avenue","East Finchley",0,51.590935,-0.153627,"102",1783956406000]
[0,"Midhurst Avenue",0]
[2,"Midhurst Avenue","East Finchley",0,51.590935,-0.153627,2,"Bus Stop Closed\nPlease use the next stop\nor the previous stop\nto catch your bus"]
[1,"Midhurst Avenue","East Finchley",0,51.590935,-0.153627,"234",1783957378000]
`

func TestGetBusDataWithStatusMessages(t *testing.T) {
	setTestLogger(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, testDataWithStatus)
	}))
	defer ts.Close()

	buses, info, err := GetBusData(ts.URL, 123)
	require.NoError(t, err)

	require.Equal(t, 2, len(buses))
	require.Equal(t, "Midhurst Avenue", info.Name)
	require.Equal(t, "East Finchley", info.Towards)
	require.Equal(t, 0, info.State)

	require.Equal(t, 1, len(info.Status))
	require.Equal(t, 2, info.Status[0].Type)
	require.Contains(t, info.Status[0].Text, "Bus Stop Closed")
	require.Contains(t, info.Status[0].Text, "Please use the next stop")
}

var testDataWithClosedStop = `
[4,"1.0",1783956388076]
[0,"Closed Stop",2]
[2,"Closed Stop","North",2,51.5,-0.15,2,"Service suspended"]
`

func TestGetBusDataWithClosedStop(t *testing.T) {
	setTestLogger(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, testDataWithClosedStop)
	}))
	defer ts.Close()

	buses, info, err := GetBusData(ts.URL, 123)
	require.NoError(t, err)

	require.Equal(t, 0, len(buses))
	require.Equal(t, "Closed Stop", info.Name)
	require.Equal(t, 2, info.State)
	require.Equal(t, 1, len(info.Status))
	require.Equal(t, "Service suspended", info.Status[0].Text)
}

var testDataWithTempClosed = `
[4,"1.0",1783956388076]
[0,"Temp Closed",1]
[2,"Temp Closed","South",1,51.5,-0.15,1,"Temporarily closed for maintenance"]
[1,"Temp Closed","South",1,51.5,-0.15,"42",1783956406000]
`

func TestGetBusDataWithTempClosedStop(t *testing.T) {
	setTestLogger(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, testDataWithTempClosed)
	}))
	defer ts.Close()

	buses, info, err := GetBusData(ts.URL, 123)
	require.NoError(t, err)

	require.Equal(t, 1, len(buses))
	require.Equal(t, "Temp Closed", info.Name)
	require.Equal(t, 1, info.State)
	require.Equal(t, 1, len(info.Status))
	require.Contains(t, info.Status[0].Text, "maintenance")
}
