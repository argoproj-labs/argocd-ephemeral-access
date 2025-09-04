package metrics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/plugin"
	"github.com/argoproj-labs/argocd-ephemeral-access/test/mocks"
	"github.com/argoproj-labs/argocd-ephemeral-access/test/utils"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
)

func TestIncrementAccessRequestCounter(t *testing.T) {
	accessRequestStatusTotal.Reset()

	expected := `
	# HELP access_request_status_total Total number of AccessRequests transitions by status
	# TYPE access_request_status_total counter
	access_request_status_total{status="denied"} 1
	access_request_status_total{status="expired"} 1
	access_request_status_total{status="granted"} 2
	access_request_status_total{status="requested"} 1
	`

	// Test different access request status values
	statuses := []api.Status{api.RequestedStatus, api.GrantedStatus, api.ExpiredStatus, api.DeniedStatus}

	// Increment each status once
	for _, status := range statuses {
		IncrementAccessRequestCounter(status)
	}

	// Increment "Granted" a second time
	IncrementAccessRequestCounter(api.GrantedStatus)

	if err := testutil.CollectAndCompare(accessRequestStatusTotal, strings.NewReader(expected), accessRequestStatusTotalMetricName); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestUpdateAccessRequests(t *testing.T) {
	readerMock := mocks.MockReader{}

	expected := `
	# HELP access_request_resources Current number of AccessRequests
	# TYPE access_request_resources gauge
	access_request_resources{role_name="role1",role_namespace="roleNs",status="expired"} 1
	access_request_resources{role_name="role1",role_namespace="roleNs",status="invalid"} 3
	access_request_resources{role_name="role2",role_namespace="roleNs",status="invalid"} 1
	`

	ar1 := utils.NewAccessRequest("ar1", "ns", "app", "appNs", "role1", "roleNs", "user-id", "username")
	ar2 := ar1.DeepCopy()
	utils.WithName("ar2")(ar2)
	utils.ToExpiredState()(ar2)
	ar3 := ar1.DeepCopy()
	utils.WithName("ar3")(ar3)
	utils.ToInvalidState()(ar3)
	ar4 := ar1.DeepCopy()
	utils.WithName("ar4")(ar4)
	utils.ToInvalidState()(ar4)
	ar5 := ar1.DeepCopy()
	utils.WithName("ar5")(ar5)
	utils.ToInvalidState()(ar5)
	ar6 := ar1.DeepCopy()
	utils.WithName("ar6")(ar6)
	utils.ToInvalidState()(ar6)
	ar6.Spec.Role.TemplateRef.Name = "role2"

	readerMock.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, ol client.ObjectList, lo ...client.ListOption) error {
		arg := ol.(*api.AccessRequestList)
		arg.Items = []api.AccessRequest{
			*ar1, *ar2, *ar3, *ar4, *ar5, *ar6,
		}
		return nil
	}).Once()

	collector := newAccessRequestCollector(t.Context(), &readerMock)

	// add a serie to the existing metric that should be removed
	accessRequestResources.Reset()
	accessRequestResources.WithLabelValues("test", "removed", "label").Set(1)

	err := utils.Eventually(func() (bool, error) {
		count := testutil.CollectAndCount(collector, accessRequestResourcesMetricName)
		return count == 3, nil
	}, 5*time.Second, time.Second)
	require.NoError(t, err)

	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected), accessRequestResourcesMetricName); err != nil {
		t.Errorf("unexpected collecting result:\n%s", err)
	}
}

func TestRecordPluginOperationResult(t *testing.T) {
	// Test different plugin operation results
	pluginOperationsTotal.Reset()

	tests := []struct {
		name           string
		operation      string
		result         interface{}
		expectedResult string
	}{
		{
			name:           "grant granted",
			operation:      "grant_access",
			result:         plugin.GrantStatusGranted,
			expectedResult: string(plugin.GrantStatusGranted),
		},
		{
			name:           "grant pending",
			operation:      "grant_access",
			result:         plugin.GrantStatusPending,
			expectedResult: string(plugin.GrantStatusPending),
		},
		{
			name:           "grant denied",
			operation:      "grant_access",
			result:         plugin.GrantStatusDenied,
			expectedResult: string(plugin.GrantStatusDenied),
		},
		{
			name:           "revoke revoked",
			operation:      "revoke_access",
			result:         plugin.RevokeStatusRevoked,
			expectedResult: string(plugin.RevokeStatusRevoked),
		},
		{
			name:           "revoke pending",
			operation:      "revoke_access",
			result:         plugin.RevokeStatusPending,
			expectedResult: string(plugin.RevokeStatusPending),
		},
		{
			name:           "error result",
			operation:      "grant_access",
			result:         errors.New("test error"),
			expectedResult: "error",
		},
		{
			name:           "unknown result",
			operation:      "grant_access",
			result:         "some string",
			expectedResult: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginOperationsTotal.Reset()
			RecordPluginOperationResult(tt.operation, tt.result)

			// Check that the counter was incremented
			count := testutil.ToFloat64(pluginOperationsTotal.WithLabelValues(tt.operation, tt.expectedResult))
			assert.Equal(t, float64(1), count, "Counter should be incremented by 1")
		})
	}
}
