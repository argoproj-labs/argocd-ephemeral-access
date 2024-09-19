package backend

import (
	"context"
	"fmt"

	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type Service interface {
	GetAccessRequest(ctx context.Context, name, namespace string) (*api.AccessRequest, error)
}

type DefaultService struct {
	k8s    Persister
	logger log.Logger
}

func NewDefaultService(c Persister, l log.Logger) *DefaultService {
	return &DefaultService{
		k8s:    c,
		logger: l,
	}
}

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
