package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/log"
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/plugin"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
)

type accessRequestCollector struct {
	reader     client.Reader
	lock       sync.RWMutex
	latestInfo []accessRequestInfo
	metric     *prometheus.GaugeVec
}

type accessRequestInfo struct {
	status        string
	roleNamespace string
	roleName      string
}

const (
	metricsCollectionInterval          = 15 * time.Second
	accessRequestResourcesMetricName   = "access_request_resources"
	accessRequestStatusTotalMetricName = "access_request_status_total"
)

var (
	register sync.Once

	accessRequestStatusTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: accessRequestStatusTotalMetricName,
			Help: "Total number of AccessRequests transitions by status",
		},
		[]string{"status"},
	)

	accessRequestResources = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: accessRequestResourcesMetricName,
			Help: "Current number of AccessRequests",
		},
		[]string{"status", "role_namespace", "role_name"},
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

func newAccessRequestCollector(ctx context.Context, reader client.Reader) prometheus.Collector {
	collector := &accessRequestCollector{
		reader:     reader,
		lock:       sync.RWMutex{},
		latestInfo: []accessRequestInfo{},
		metric:     accessRequestResources,
	}
	go collector.run(ctx)
	return collector
}

func Register(ctx context.Context, reader client.Reader) {
	register.Do(func() {
		metrics.Registry.MustRegister(accessRequestStatusTotal)
		metrics.Registry.MustRegister(pluginOperationsTotal)
		metrics.Registry.MustRegister(newAccessRequestCollector(ctx, reader))
	})
}

func (c *accessRequestCollector) run(ctx context.Context) {
	c.updateData(ctx)
	tick := time.Tick(metricsCollectionInterval)
	for {
		select {
		case <-ctx.Done():
		case <-tick:
			c.updateData(ctx)
		}
	}
}

func (c *accessRequestCollector) updateData(ctx context.Context) {
	logger := log.FromContext(ctx)
	logger.Debug("Updating access_request_resources")

	list := &api.AccessRequestList{}
	err := c.reader.List(ctx, list)
	if err != nil {
		logger.Error(err, "could not list access request for metrics")
		return
	}

	newInfo := []accessRequestInfo{}
	for _, ar := range list.Items {
		status := string(ar.Status.RequestState)
		if status == "" {
			continue
		}
		info := accessRequestInfo{
			status:        status,
			roleNamespace: ar.Spec.Role.TemplateRef.Namespace,
			roleName:      ar.Spec.Role.TemplateRef.Name,
		}
		newInfo = append(newInfo, info)
	}

	c.lock.Lock()
	c.latestInfo = newInfo
	c.lock.Unlock()
}

// Describe implements the prometheus.Collector interface
func (c *accessRequestCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metric.Describe(ch)
}

func (c *accessRequestCollector) Collect(ch chan<- prometheus.Metric) {
	c.lock.RLock()
	latestInfo := c.latestInfo
	c.lock.RUnlock()

	c.metric.Reset()
	for _, info := range latestInfo {
		c.metric.WithLabelValues(info.status, info.roleNamespace, info.roleName).Inc()
	}

	c.metric.Collect(ch)
}

// IncrementAccessRequestCounter increments the counter for a given AccessRequest status
func IncrementAccessRequestCounter(status api.Status) {
	accessRequestStatusTotal.WithLabelValues(string(status)).Inc()
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
