// metrics.go defines and registers all Prometheus metrics for the bus display.
package main

import (
	"regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	metricTFLRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bus_tfl_requests_total",
		Help: "Total number of TFL API requests.",
	}, []string{"result"}) // result: "ok" or "error"

	metricTFLBuses = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "bus_tfl_buses",
		Help: "Number of buses returned in the last TFL API response.",
	})

	metricTFLClockSkew = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "bus_tfl_clock_skew_seconds",
		Help: "Clock skew between TFL server and local machine in seconds (signed).",
	})

	metricWeatherRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "bus_weather_requests_total",
		Help: "Total number of weather API requests.",
	}, []string{"result"}) // result: "ok" or "error"

	metricFrameRender = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "bus_frame_render_seconds",
		Help:    "Time to render a single display frame.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
	})
)

func init() {
	// Unregister the default collectors so we can replace the Go collector
	// with one that exposes all runtime metrics.
	prometheus.Unregister(collectors.NewGoCollector())
	prometheus.Unregister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	prometheus.MustRegister(
		metricTFLRequests,
		metricTFLBuses,
		metricTFLClockSkew,
		metricWeatherRequests,
		metricFrameRender,
		collectors.NewGoCollector(
			collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{
				Matcher: regexp.MustCompile("/.+"),
			}),
		),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}
