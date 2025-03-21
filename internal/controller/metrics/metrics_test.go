package metrics

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestIncrementAccessRequestCounter(t *testing.T) {
	accessRequestStatusTotal.Reset()

	// Test different access request status values
	statuses := []string{"Requested", "Granted", "Expired", "Denied"}

	// Increment each status once
	for _, status := range statuses {
		IncrementAccessRequestCounter(status)
	}

	// Increment "Granted" a second time
	IncrementAccessRequestCounter("Granted")

	// Check all metrics have expected values
	for _, status := range statuses {
		expected := float64(1)
		if status == "Granted" {
			expected = 2
		}

		metric := &dto.Metric{}
		err := accessRequestStatusTotal.WithLabelValues(status).Write(metric)
		assert.NoError(t, err, "Failed to write metric for status %s", status)

		count := metric.Counter.GetValue()
		assert.Equal(t, expected, count, "Incorrect count for status %s", status)
	}
}
