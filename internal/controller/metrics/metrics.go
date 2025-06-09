package metrics

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/log"
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/plugin"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
)

const (
	accessRequestsUpdateMaxFrequency   = 15 * time.Second
	accessRequestResourcesMetricName   = "access_request_resources"
	accessRequestStatusTotalMetricName = "access_request_status_total"
)

var (
	accessRequestStatusTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: accessRequestStatusTotalMetricName,
			Help: "Total number of AccessRequests transitions by status",
		},
		[]string{"status"},
	)

	accessRequestResources = newThrottledGauge(accessRequestsUpdateMaxFrequency, prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: accessRequestResourcesMetricName,
			Help: "Current number of AccessRequests",
		},
		[]string{"status", "roleNamespace", "roleName"},
	))

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
	metrics.Registry.MustRegister(accessRequestResources)
	metrics.Registry.MustRegister(pluginOperationsTotal)
}

// IncrementAccessRequestCounter increments the counter for a given AccessRequest status
func IncrementAccessRequestCounter(status api.Status) {
	accessRequestStatusTotal.WithLabelValues(string(status)).Inc()
}

// UpdateAccessRequests increments the gauge based on the Access Requests
func UpdateAccessRequests(reader client.Reader) {
	accessRequestResources.run(func(m *prometheus.GaugeVec) {
		ctx := context.Background()
		logger := log.FromContext(ctx)
		logger.Debug("Updating access_request_resources")

		list := &api.AccessRequestList{}
		err := reader.List(ctx, list)
		if err != nil {
			logger.Error(err, "could not list access request for metrics")
		}

		countByKey := map[string]int{}
		for _, ar := range list.Items {
			roleName := ar.Spec.Role.TemplateRef.Name
			roleNamespace := ar.Spec.Role.TemplateRef.Namespace
			status := string(ar.Status.RequestState)
			if status == "" {
				continue
			}

			key := fmt.Sprintf("%s/%s/%s", status, roleNamespace, roleName)
			countByKey[key] += 1
		}

		m.Reset()
		for key, count := range countByKey {
			labels := strings.Split(key, "/")
			m.WithLabelValues(labels...).Set(float64(count))
		}
	}, false)
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

type throttledGauge struct {
	*prometheus.GaugeVec

	delay time.Duration

	lastCallTime time.Time
	callAfter    bool
	mutex        *sync.Mutex
}

func newThrottledGauge(delay time.Duration, gauge *prometheus.GaugeVec) *throttledGauge {
	return &throttledGauge{
		GaugeVec: gauge,
		delay:    delay,
		mutex:    &sync.Mutex{},
	}
}

func (c *throttledGauge) run(fn func(*prometheus.GaugeVec), throttled bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if time.Since(c.lastCallTime) > c.delay {
		go fn(c.GaugeVec)
		c.lastCallTime = time.Now()
		c.callAfter = false
	} else if !c.callAfter && !throttled {
		// Queue the operation after the delay has expired in case no more event comes in
		// If there is already an item queued, then we can drop the subsequent events
		c.callAfter = true
		time.AfterFunc(time.Until(c.lastCallTime.Add(c.delay).Add(time.Millisecond)), func() { c.run(fn, true) })
	}
}
