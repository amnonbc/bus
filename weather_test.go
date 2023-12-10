package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetWeather(t *testing.T) {
	w, err := GetWeather()
	require.NoError(t, err)
	require.NotEmpty(t, w)
	t.Log(w[0].String())
}

func TestGetLocationCode(t *testing.T) {
	c, err := GetLocationCode("N2 9LU")
	require.NoError(t, err)
	t.Log(c)
}
