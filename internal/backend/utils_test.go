package backend_test

import (
	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/argocd-ephemeral-access/internal/backend"
	"github.com/argoproj-labs/argocd-ephemeral-access/test/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func newAccessRequest(key *backend.AccessRequestKey, roleName string) *api.AccessRequest {
	return utils.NewAccessRequest("test-acccess-request", key.Namespace, key.ApplicationName, key.ApplicationNamespace, roleName, key.Namespace, key.UserId, key.Username)
}

func newDefaultAccessBinding() *api.AccessBinding {
	return newAccessBinding("test-ns", "test-role", "")
}

func newAccessBinding(namespace, roleName, allowedSubject string) *api.AccessBinding {
	subjects := []string{}
	if allowedSubject != "" {
		subjects = append(subjects, allowedSubject)
	}

	return &api.AccessBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AccessBinding",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ab",
			Namespace: namespace,
		},
		Spec: api.AccessBindingSpec{
			RoleTemplateRef: api.RoleTemplateReference{
				Name: roleName,
			},
			FriendlyName: ptr.To("Default Test Role"),
			Ordinal:      99,
			Subjects:     subjects,
		},
	}
}
