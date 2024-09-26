package backend

import (
	"context"
	"fmt"

	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Service defines the operations provided by the backend. Backend business
// logic should be added in implementations of this interface
type Service interface {
	GetAccessRequest(ctx context.Context, name, namespace string) (*api.AccessRequest, error)
	CreateAccessRequest(ctx context.Context, ar *api.AccessRequest) (*api.AccessRequest, error)

	GetAccessBindings(ctx context.Context, name, namespace string) ([]*api.AccessBinding, error)

	GetApplication(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error)
	GetAppProject(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error)
}

// DefaultService is the real Service implementation
type DefaultService struct {
	k8s    Persister
	logger log.Logger
}

// NewDefaultService will return a new DefaultService instance.
func NewDefaultService(c Persister, l log.Logger) *DefaultService {
	return &DefaultService{
		k8s:    c,
		logger: l,
	}
}

// GetAccessRequest will retrieve the access request from k8s identified by the
// given name and namespace. Will return a nil value without any error if the
// access request isn't found.
func (s *DefaultService) GetAccessRequest(ctx context.Context, name, namespace string) (*api.AccessRequest, error) {
	ar, err := s.k8s.GetAccessRequest(ctx, name, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		s.logger.Error(err, "error getting accessrequest from k8s")
		return nil, fmt.Errorf("error getting accessrequest from k8s: %w", err)
	}
	return ar, nil
}

// CreateAccessRequest implements Service.
func (s *DefaultService) CreateAccessRequest(ctx context.Context, ar *api.AccessRequest) (*api.AccessRequest, error) {
	panic("TODO: unimplemented")
}

// GetAccessBindings implements Service.
func (s *DefaultService) GetAccessBindings(ctx context.Context, name string, namespace string) ([]*api.AccessBinding, error) {
	panic("TODO: unimplemented")
}

// GetAppProject implements Service.
func (s *DefaultService) GetAppProject(ctx context.Context, name string, namespace string) (*unstructured.Unstructured, error) {
	panic("TODO: unimplemented")
}

// GetApplication implements Service.
func (s *DefaultService) GetApplication(ctx context.Context, name string, namespace string) (*unstructured.Unstructured, error) {
	panic("TODO: unimplemented")
}
