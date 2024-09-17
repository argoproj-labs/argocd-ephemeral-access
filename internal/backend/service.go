package backend

import (
	"context"
	"fmt"

	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type Service interface {
	GetAccessRequest(ctx context.Context, name, namespace string) (*api.AccessRequest, error)
}

type DefaultService struct {
	client *dynamic.DynamicClient
	logger log.Logger
}

func NewDefaultService(c *dynamic.DynamicClient, l log.Logger) *DefaultService {
	return &DefaultService{
		client: c,
		logger: l,
	}
}

func (s *DefaultService) GetAccessRequest(ctx context.Context, name, namespace string) (*api.AccessRequest, error) {
	resource := schema.GroupVersionResource{
		Group:    api.GroupVersion.Group,
		Version:  api.GroupVersion.Version,
		Resource: "accessrequests",
	}
	resp, err := s.client.Resource(resource).
		Namespace(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting accessrequest from k8s: %w", err)
	}

	u := resp.UnstructuredContent()
	var ar api.AccessRequest
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u, &ar)
	if err != nil {
		return nil, fmt.Errorf("error converting accessrequest unstructured: %w", err)
	}

	return &ar, nil
}
