package metrics

import (
	"net/http"
	"strconv"
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
		[]string{"method", "path", "status"},
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
func recordAPIRequest(method, path string, duration float64, statusCode int) {
	apiRequestsTotal.WithLabelValues(method, path, strconv.Itoa(statusCode)).Inc()
	apiRequestDuration.WithLabelValues(method, path).Observe(duration)
}

// MetricsMiddleware is a middleware function that records API request metrics.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapper := &responseWriterWrapper{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(wrapper, r)
		// Get duration in milliseconds
		duration := float64(time.Since(start).Milliseconds())

		recordAPIRequest(r.Method, r.URL.Path, duration, wrapper.statusCode)
	})
}

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
