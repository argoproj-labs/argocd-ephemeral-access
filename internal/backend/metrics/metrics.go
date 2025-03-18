package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
	// Metrics defined here
	apiRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "path"},
	)

	apiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_seconds",
			Help:    "Duration of API requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	// Register the metrics
	prometheus.MustRegister(apiRequestsTotal)
	prometheus.MustRegister(apiRequestDuration)
}

// MetricsHandler returns an http.Handler for the metrics endpoint
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RecordAPIRequest records metrics for an API request
func RecordAPIRequest(method, path string, duration float64) {
	apiRequestsTotal.WithLabelValues(method, path).Inc()
	apiRequestDuration.WithLabelValues(method, path).Observe(duration)
}
