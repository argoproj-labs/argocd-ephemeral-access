package backend_test

import (
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/argoproj-labs/ephemeral-access/test/utils"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newAccessRequest(key *backend.AccessRequestKey, roleName string) *api.AccessRequest {
	return utils.NewAccessRequest("test-acccess-request", key.Namespace, key.ApplicationName, key.ApplicationNamespace, roleName, key.Username)
}

func newAccessBinding(allowedSubject string) *api.AccessBinding {
	return &api.AccessBinding{
		TypeMeta: v1.TypeMeta{
			Kind:       "AccessBinding",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "allowed",
			Namespace: "namespace",
		},
		Spec: api.AccessBindingSpec{
			RoleTemplateRef: api.RoleTemplateReference{
				Name: "some-template",
			},
			Subjects: []string{
				allowedSubject,
			},
		},
	}
}
