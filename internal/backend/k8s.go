package backend

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
)

const (
	resourceType = "accessrequests"
)

// Persister defines the operations to interact with the backend persistent
// layer (e.g. Kubernetes)
type Persister interface {
	GetAccessRequest(ctx context.Context, name, namespace string) (*api.AccessRequest, error)
}

// K8sPersister is a K8s implementation for the Persister interface.
type K8sPersister struct {
	client dynamic.Interface
}

// NewK8sPersister will return a new K8sPersister instance.
func NewK8sPersister(c dynamic.Interface) *K8sPersister {
	return &K8sPersister{
		client: c,
	}
}

// GetAccessRequestResource return a GroupVersionResource schema for the
// AccessRequest CRD.
func GetAccessRequestResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    api.GroupVersion.Group,
		Version:  api.GroupVersion.Version,
		Resource: resourceType,
	}
}

// GetAccessRequest will retrieve an AccessRequest from k8s identified by the given
// name and namespace.
func (c *K8sPersister) GetAccessRequest(ctx context.Context, name, namespace string) (*api.AccessRequest, error) {
	resp, err := c.client.Resource(GetAccessRequestResource()).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error retrieving resource from k8s: %w", err)
	}
	u := resp.UnstructuredContent()
	var ar api.AccessRequest
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u, &ar)
	if err != nil {
		return nil, fmt.Errorf("error converting accessrequest unstructured: %w", err)
	}
	return &ar, nil
}
