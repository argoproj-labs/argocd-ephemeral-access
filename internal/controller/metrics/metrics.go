package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	accessRequestStatusTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "access_request_status_total",
			Help: "Total number of AccessRequests by status",
		},
		[]string{"accessRequestStatus"},
	)
)

func init() {
	metrics.Registry.MustRegister(accessRequestStatusTotal)
}

// IncrementAccessRequestCounter increments the counter for a given AccessRequest status
func IncrementAccessRequestCounter(status string) {
	accessRequestStatusTotal.WithLabelValues(status).Inc()
}
