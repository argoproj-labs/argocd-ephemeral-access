package backend_test

import (
	"context"
	"fmt"
	"testing"

	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

const (
	accessRequestKind = "AccessRequest"
)

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}

func newAccessRequestUnstructured(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": fmt.Sprintf("%s/%s", api.GroupVersion.Group, api.GroupVersion.Version),
			"kind":       accessRequestKind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
}

func addSpec(spec map[string]interface{}, u *unstructured.Unstructured) {
	u.Object["spec"] = spec
}

func TestK8sGetAccessRequest(t *testing.T) {
	t.Run("will retrieve AccessRequest successfully", func(t *testing.T) {
		// Given
		arName := "some-ar"
		namespace := "some-ns"
		appName := "some-application"
		appNamespace := "argocd"
		ar := newAccessRequestUnstructured(arName, namespace)
		app := map[string]interface{}{"name": appName, "namespace": appNamespace}
		spec := map[string]interface{}{"duration": "1m", "roleTemplateName": "role-template", "application": app}
		addSpec(spec, ar)
		dc := fake.NewSimpleDynamicClient(runtime.NewScheme(), ar)
		p := backend.NewK8sPersister(dc)

		// When
		result, err := p.GetAccessRequest(context.Background(), arName, namespace)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, arName, result.GetName())
		assert.Equal(t, namespace, result.GetNamespace())
		assert.Equal(t, appName, result.Spec.Application.Name)
		assert.Equal(t, appNamespace, result.Spec.Application.Namespace)
	})
	t.Run("will return error if ar not found", func(t *testing.T) {
		// Given
		arName := "some-ar"
		namespace := "some-ns"
		appName := "some-application"
		appNamespace := "argocd"
		ar := newAccessRequestUnstructured(arName, namespace)
		app := map[string]interface{}{"name": appName, "namespace": appNamespace}
		spec := map[string]interface{}{"duration": "1m", "roleTemplateName": "role-template", "application": app}
		addSpec(spec, ar)
		dc := fake.NewSimpleDynamicClient(runtime.NewScheme(), ar)
		p := backend.NewK8sPersister(dc)

		// When
		result, err := p.GetAccessRequest(context.Background(), "NOTFOUND", namespace)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "\"NOTFOUND\" not found")
	})

}
