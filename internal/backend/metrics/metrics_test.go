package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestRecordAPIRequest(t *testing.T) {
	method := "GET"
	path := "/accessrequests"
	duration := 123.0

	recordAPIRequest(method, path, duration)
	assert.Equal(t, 1, testutil.CollectAndCount(apiRequestsTotal, "api_requests_total"))
	assert.Equal(t, 1, testutil.CollectAndCount(apiRequestDuration, "api_request_duration_milliseconds"))
}

func TestMetricsMiddleware(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		time.Sleep(50 * time.Millisecond)
	})
	testHandler := MetricsMiddleware(nextHandler)

	req := httptest.NewRequest("GET", "/accessrequests", nil)
	rec := httptest.NewRecorder()

	testHandler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// Check if metrics have been recorded
	assert.Equal(t, 1, testutil.CollectAndCount(apiRequestsTotal, "api_requests_total"))
	assert.True(t, testutil.CollectAndCount(apiRequestDuration, "api_request_duration_milliseconds") > 0)
}
