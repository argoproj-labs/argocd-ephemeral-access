package controller

import (
	"testing"

	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestSubjectRBACIdentity(t *testing.T) {
	t.Run("prefers Argo CD user ID", func(t *testing.T) {
		ar := &api.AccessRequest{Spec: api.AccessRequestSpec{Subject: api.Subject{
			Username: "engineer@example.com",
			UserId:   ptr.To("12345"),
		}}}

		assert.Equal(t, "12345", subjectRBACIdentity(ar))
	})

	t.Run("falls back to username for Argo CD versions without a user ID", func(t *testing.T) {
		ar := &api.AccessRequest{Spec: api.AccessRequestSpec{Subject: api.Subject{
			Username: "engineer@example.com",
		}}}

		assert.Equal(t, "engineer@example.com", subjectRBACIdentity(ar))
	})
}
