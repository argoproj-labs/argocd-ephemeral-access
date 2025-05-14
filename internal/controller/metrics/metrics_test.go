package metrics

import (
	"fmt"
	"testing"

	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/plugin"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/stretchr/testify/assert"
)

func TestIncrementAccessRequestCounter(t *testing.T) {
	accessRequestResources.Reset()
	//TODO
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
			result:         fmt.Errorf("test error"),
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
