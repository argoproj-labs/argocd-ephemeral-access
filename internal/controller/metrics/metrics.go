package metrics

import (
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/plugin"
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

	// PluginOperationsTotal counts the total number of plugin operations. The plugin operation can be either
	// revoke_access or grant_access.
	pluginOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "plugin_operations_total",
			Help: "Total number of plugin operations by type and result",
		},
		[]string{"operation", "result"},
	)
)

func init() {
	metrics.Registry.MustRegister(accessRequestStatusTotal)
	metrics.Registry.MustRegister(pluginOperationsTotal)
}

// IncrementAccessRequestCounter increments the counter for a given AccessRequest status
func IncrementAccessRequestCounter(status string) {
	accessRequestStatusTotal.WithLabelValues(status).Inc()
}

// RecordPluginOperationResult records the result of a plugin operation
func RecordPluginOperationResult(operation string, result interface{}) {
	var resultString string
	switch r := result.(type) {
	case plugin.GrantStatus:
		resultString = string(r)
	case plugin.RevokeStatus:
		resultString = string(r)
	case error:
		resultString = "error"
	default:
		resultString = "unknown"
	}
	pluginOperationsTotal.WithLabelValues(operation, resultString).Inc()
}
