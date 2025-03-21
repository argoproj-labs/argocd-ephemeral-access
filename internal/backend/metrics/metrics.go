package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	apiRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"method", "path"},
	)

	apiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "api_request_duration_milliseconds",
			Help:    "Duration of API requests in milliseconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	prometheus.MustRegister(apiRequestsTotal)
	prometheus.MustRegister(apiRequestDuration)
}

// MetricsHandler returns an http.Handler for the metrics endpoint
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RecordAPIRequest records metrics for an API request
func recordAPIRequest(method, path string, duration float64) {
	apiRequestsTotal.WithLabelValues(method, path).Inc()
	apiRequestDuration.WithLabelValues(method, path).Observe(duration)
}

// MetricsMiddleware is a middleware function that records API request metrics.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		// Get duration in milliseconds
		duration := float64(time.Since(start).Milliseconds())
		recordAPIRequest(r.Method, r.URL.Path, duration)
	})
}
