package backend

import (
	"context"
	"fmt"

	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Service defines the operations provided by the backend. Backend business
// logic should be added in implementations of this interface
type Service interface {
	CreateAccessRequest(ctx context.Context, key *AccessRequestKey, binding *api.AccessBinding) (*api.AccessRequest, error)
	GetAccessRequest(ctx context.Context, key *AccessRequestKey, roleName string) (*api.AccessRequest, error)
	ListAccessRequests(ctx context.Context, key *AccessRequestKey) ([]*api.AccessRequest, error)

	GetGrantingAccessBinding(ctx context.Context, roleName string, namespace string, groups []string, app *unstructured.Unstructured, project *unstructured.Unstructured) (*api.AccessBinding, error)

	GetApplication(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error)
	GetAppProject(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error)
}

type AccessRequestKey struct {
	Namespace            string
	ApplicationName      string
	ApplicationNamespace string
	Username             string
}

func (k *AccessRequestKey) ResourceName(roleName string) string {
	//TODO: hash it and validate k8s max length
	return fmt.Sprintf("%s-%s-%s-%s", k.ApplicationNamespace, k.ApplicationName, roleName, k.Username)
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
func (s *DefaultService) GetAccessRequest(ctx context.Context, key *AccessRequestKey, roleName string) (*api.AccessRequest, error) {
	//TODO: this only works if we expect resource to be unique
	ar, err := s.k8s.GetAccessRequest(ctx, key.ResourceName(roleName), key.Namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		s.logger.Error(err, "error getting accessrequest from k8s")
		return nil, fmt.Errorf("error getting accessrequest from k8s: %w", err)
	}
	return ar, nil
}

func (s *DefaultService) ListAccessRequests(ctx context.Context, key *AccessRequestKey) ([]*api.AccessRequest, error) {
	accessRequests, err := s.k8s.ListAccessRequests(ctx, key.Namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		s.logger.Error(err, "error getting accessrequest from k8s")
		return nil, fmt.Errorf("error getting accessrequest from k8s: %w", err)
	}

	//TODO: k8s.ListAccessRequests should be optimized to filter on some fields
	filtered := []*api.AccessRequest{}
	for i, ar := range accessRequests {
		if ar.Spec.Subject.Username != key.Username {
			continue
		}
		if ar.Spec.Application.Name != key.ApplicationName {
			continue
		}
		if ar.Spec.Application.Namespace != key.ApplicationNamespace {
			continue
		}
		filtered = append(filtered, accessRequests[i])
	}

	return filtered, nil
}

func (s *DefaultService) GetGrantingAccessBinding(ctx context.Context, roleName string, namespace string, groups []string, app *unstructured.Unstructured, project *unstructured.Unstructured) (*api.AccessBinding, error) {
	bindings, err := s.getAccessBindings(ctx, roleName, namespace)
	if err != nil {
		s.logger.Error(err, "error getting access bindings")
		return nil, fmt.Errorf("error retrieving access bindings for role %s: %w", roleName, err)
	}

	if len(bindings) == 0 {
		return nil, nil
	}

	s.logger.Debug(fmt.Sprintf("found %d bindings referencing role %s", len(bindings), roleName))
	var grantingBinding *api.AccessBinding
	for i, binding := range bindings {

		subjects, err := binding.RenderSubjects(app, project)
		if err != nil {
			s.logger.Error(err, fmt.Sprintf("cannot render subjects %s:", binding.Name))
			continue
		}

		if s.matchSubject(subjects, groups) {
			grantingBinding = bindings[i]
			break
		}
	}

	return grantingBinding, nil
}

func (s *DefaultService) matchSubject(subjects, groups []string) bool {
	for _, subject := range subjects {
		for _, g := range groups {
			if subject == g {
				return true
			}
		}
	}
	return false
}

// CreateAccessRequest implements Service.
func (s *DefaultService) CreateAccessRequest(ctx context.Context, key *AccessRequestKey, binding *api.AccessBinding) (*api.AccessRequest, error) {
	roleName := binding.Spec.RoleTemplateRef.Name
	ar := &api.AccessRequest{
		ObjectMeta: v1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.ResourceName(roleName),
		},
		Spec: api.AccessRequestSpec{
			RoleTemplateName: roleName,
			Subject: api.Subject{
				Username: key.Username,
			},
			Application: api.TargetApplication{
				Name:      key.ApplicationName,
				Namespace: key.ApplicationNamespace,
			},
		},
	}
	//TODO: Set duration. Configurable by the users? Server Config?
	ar, err := s.k8s.CreateAccessRequest(ctx, ar)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		s.logger.Error(err, "error getting accessrequest from k8s")
		return nil, fmt.Errorf("error getting accessrequest from k8s: %w", err)
	}
	return ar, nil
}

// GetAppProject implements Service.
func (s *DefaultService) GetAppProject(ctx context.Context, name string, namespace string) (*unstructured.Unstructured, error) {
	panic("TODO: unimplemented")
}

// GetApplication implements Service.
func (s *DefaultService) GetApplication(ctx context.Context, name string, namespace string) (*unstructured.Unstructured, error) {
	panic("TODO: unimplemented")
}

// GetAccessBindings implements Service.
func (s *DefaultService) getAccessBindings(ctx context.Context, name string, namespace string) ([]*api.AccessBinding, error) {
	// Should get all the binding in namespace AND all bindings in controller namespace
	panic("TODO: unimplemented")
}
