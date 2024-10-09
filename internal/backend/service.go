package backend

import (
	"context"
	"fmt"
	"slices"
	"strings"

	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Service defines the operations provided by the backend. Backend business
// logic should be added in implementations of this interface
type Service interface {
	CreateAccessRequest(ctx context.Context, key *AccessRequestKey, binding *api.AccessBinding) (*api.AccessRequest, error)
	GetAccessRequestByRole(ctx context.Context, key *AccessRequestKey, roleName string) (*api.AccessRequest, error)
	ListAccessRequests(ctx context.Context, key *AccessRequestKey, sort bool) ([]*api.AccessRequest, error)

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

// DefaultService is the real Service implementation
type DefaultService struct {
	k8s       Persister
	logger    log.Logger
	namespace string
}

var requestStateOrder = map[api.Status]int{
	// empty is the default and assumed to be the same as requested
	"":                  0,
	api.RequestedStatus: 0,
	api.GrantedStatus:   1,
	api.DeniedStatus:    2,
	api.InvalidStatus:   3,
	api.ExpiredStatus:   4,
}

// NewDefaultService will return a new DefaultService instance.
func NewDefaultService(c Persister, l log.Logger, namespace string) *DefaultService {
	return &DefaultService{
		k8s:       c,
		logger:    l,
		namespace: namespace,
	}
}

// GetAccessRequestByRole will retrieve the access request for the specified role.
// Will return a nil value without any error if an access request isn't found for this role.
func (s *DefaultService) GetAccessRequestByRole(ctx context.Context, key *AccessRequestKey, roleName string) (*api.AccessRequest, error) {

	// get all access requests
	accessRequests, err := s.ListAccessRequests(ctx, key, true)
	if err != nil {
		return nil, fmt.Errorf("error listing access request for role %s: %w", roleName, err)
	}

	// find the first access request matching the requested role
	for _, ar := range accessRequests {
		if ar.Spec.Role.TemplateName == roleName {
			return ar, nil
		}
	}

	return nil, nil
}

// ListAccessRequests will list non-expired access requests and optionally sort them by importance
func (s *DefaultService) ListAccessRequests(ctx context.Context, key *AccessRequestKey, shouldSort bool) ([]*api.AccessRequest, error) {
	accessRequests, err := s.k8s.ListAccessRequests(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("error getting accessrequest from k8s: %w", err)
	}

	filtered := []*api.AccessRequest{}
	for i, ar := range accessRequests.Items {
		if ar.Status.RequestState == api.ExpiredStatus {
			// ignore expired request
			continue
		}
		filtered = append(filtered, &accessRequests.Items[i])
	}

	if shouldSort {
		slices.SortStableFunc(filtered, defaultAccessRequestSort)
	}

	return filtered, nil
}

// GetGrantingAccessBinding will return the first AccessBinding allowing at least one of the group to request the specified role
// AccessBinding can be located in the specified namespace or in the controller namespace.
// If no bindings are granting access, nil is returned
func (s *DefaultService) GetGrantingAccessBinding(ctx context.Context, roleName string, namespace string, groups []string, app *unstructured.Unstructured, project *unstructured.Unstructured) (*api.AccessBinding, error) {
	bindings, err := s.listAccessBindings(ctx, roleName, namespace)
	if err != nil {
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
			grantingBinding = &bindings[i]
			break
		}
	}

	return grantingBinding, nil
}

// matchSubject returns true if groups contains at least one of subjects
func (s *DefaultService) matchSubject(subjects, groups []string) bool {
	for _, subject := range subjects {
		if found := slices.Contains(groups, subject); found {
			return true
		}
	}
	return false
}

// CreateAccessRequest willl crate an AccessRequest for the given key requesting the role speified by the AccessBinding
func (s *DefaultService) CreateAccessRequest(ctx context.Context, key *AccessRequestKey, binding *api.AccessBinding) (*api.AccessRequest, error) {
	roleName := binding.Spec.RoleTemplateRef.Name
	ar := &api.AccessRequest{
		ObjectMeta: v1.ObjectMeta{
			Namespace:    key.Namespace,
			GenerateName: s.getAccessRequestPrefix(key.Username, roleName),
		},
		Spec: api.AccessRequestSpec{
			Role: api.TargetRole{
				TemplateName: binding.Spec.RoleTemplateRef.Name,
				Ordinal:      binding.Spec.Ordinal,
				FriendlyName: binding.Spec.FriendlyName,
			},
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
		return nil, fmt.Errorf("error creating access request from k8s: %w", err)
	}
	return ar, nil
}

func (s *DefaultService) getAccessRequestPrefix(username, roleName string) string {
	prefix := strings.ToLower(fmt.Sprintf("%s-", "TODO"))
	if len(validation.NameIsDNSSubdomain(prefix, true)) != 0 {
		prefix = strings.ToLower("TODO-fallback-")
	}
	return prefix
}

func (s *DefaultService) GetApplication(ctx context.Context, name string, namespace string) (*unstructured.Unstructured, error) {
	app, err := s.k8s.GetApplication(ctx, name, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return app, nil
}

func (s *DefaultService) GetAppProject(ctx context.Context, name string, namespace string) (*unstructured.Unstructured, error) {
	project, err := s.k8s.GetAppProject(ctx, name, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return project, nil
}

func (s *DefaultService) listAccessBindings(ctx context.Context, roleName string, namespace string) ([]api.AccessBinding, error) {
	// get all the binding in argo namespace
	namespacedBindings, err := s.k8s.ListAccessBindings(ctx, roleName, namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting accessrequest from k8s: %w", err)
	}
	// get all the binding in controller namespace
	globalBindings, err := s.k8s.ListAccessBindings(ctx, roleName, s.namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting accessrequest from k8s: %w", err)
	}
	return append(namespacedBindings.Items, globalBindings.Items...), nil
}

func defaultAccessRequestSort(a, b *api.AccessRequest) int {
	// sort by status
	if a.Status.RequestState != b.Status.RequestState {
		aOrder := requestStateOrder[a.Status.RequestState]
		bOrder := requestStateOrder[b.Status.RequestState]
		return aOrder - bOrder
	}

	// sort by ordinal ascending
	if a.Spec.Role.Ordinal != b.Spec.Role.Ordinal {
		return a.Spec.Role.Ordinal - b.Spec.Role.Ordinal
	}

	// sort by role name ascending
	if a.Spec.Role.TemplateName != b.Spec.Role.TemplateName {
		return strings.Compare(a.Spec.Role.TemplateName, b.Spec.Role.TemplateName)
	}

	// sort by creation date. Priority to newer request
	if a.CreationTimestamp != b.CreationTimestamp {
		if a.CreationTimestamp.Before(&b.CreationTimestamp) {
			return -1
		}
		return +1
	}

	return 0
}
