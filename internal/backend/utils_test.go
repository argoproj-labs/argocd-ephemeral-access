package backend_test

import (
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/argoproj-labs/ephemeral-access/test/utils"
)

func newAccessRequest(key *backend.AccessRequestKey, roleName string) *api.AccessRequest {
	return utils.NewAccessRequest("test-acccess-request", key.Namespace, key.ApplicationName, key.ApplicationNamespace, roleName, "")
}
