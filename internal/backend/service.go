package backend

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/backend/generator"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Service defines the operations provided by the backend. Backend business
// logic should be added in implementations of this interface
type Service interface {
	// CreateAccessRequest will create an AccessRequest for the given key requesting the role specified by the AccessBinding
	CreateAccessRequest(ctx context.Context, key *AccessRequestKey, binding *api.AccessBinding) (*api.AccessRequest, error)
	// GetAccessRequestByRole will retrieve the access request for the specified role.
	// Will return a nil value without any error if an access request isn't found for this role.
	GetAccessRequestByRole(ctx context.Context, key *AccessRequestKey, roleName string) (*api.AccessRequest, error)
	// ListAccessRequests will list non-expired access requests and optionally sort them by importance.
	// The importance sort is based on status, role ordinal, name and creation date.
	ListAccessRequests(ctx context.Context, key *AccessRequestKey, sort bool) ([]*api.AccessRequest, error)

	// GetGrantingAccessBinding will return the first AccessBinding allowing at least one of the group to request the specified role
	// AccessBinding can be located in the specified namespace or in the controller namespace.
	// If no bindings are granting access, nil is returned
	GetGrantingAccessBinding(ctx context.Context, roleName string, namespace string, groups []string, app *unstructured.Unstructured, project *unstructured.Unstructured) (*api.AccessBinding, error)

	// GetApplication returns the Unstructured object representing the application. The Unstructured object
	// can be used to evaluate granting AccessBinding.
	GetApplication(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error)
	// GetAppProject returns the Unstructured object representing the app project. The Unstructured object
	// can be used to evaluate granting AccessBinding.
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
	k8s                   Persister
	logger                log.Logger
	namespace             string
	accessRequestDuration time.Duration
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

const (
	// Same as https://github.com/kubernetes/apiserver/blob/v0.31.1/pkg/storage/names/generate.go#L46
	maxNameLength          = 63
	randomLength           = 5
	MaxGeneratedNameLength = maxNameLength - randomLength
)

// NewDefaultService will return a new DefaultService instance.
func NewDefaultService(c Persister, l log.Logger, namespace string, arDuration time.Duration) *DefaultService {
	return &DefaultService{
		k8s:                   c,
		logger:                l,
		namespace:             namespace,
		accessRequestDuration: arDuration,
	}
}

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

func (s *DefaultService) CreateAccessRequest(ctx context.Context, key *AccessRequestKey, binding *api.AccessBinding) (*api.AccessRequest, error) {
	roleName := binding.Spec.RoleTemplateRef.Name
	ar := &api.AccessRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    key.Namespace,
			GenerateName: getAccessRequestPrefix(key.Username, roleName),
		},
		Spec: api.AccessRequestSpec{
			Duration: metav1.Duration{
				Duration: s.accessRequestDuration,
			},
			Role: api.TargetRole{
				TemplateName: binding.Spec.RoleTemplateRef.Name,
				Ordinal:      binding.Spec.Ordinal,
				FriendlyName: binding.Spec.FriendlyName,
			},
			Application: api.TargetApplication{
				Name:      key.ApplicationName,
				Namespace: key.ApplicationNamespace,
			},
			Subject: api.Subject{
				Username: key.Username,
			},
		},
	}
	ar, err := s.k8s.CreateAccessRequest(ctx, ar)
	if err != nil {
		return nil, fmt.Errorf("error creating access request from k8s: %w", err)
	}
	return ar, nil
}

func getAccessRequestPrefix(username, roleName string) string {
	// If username is an email, we don't care about the email domain
	username, _, _ = strings.Cut(username, "@")

	username = generator.ToDNS1123Subdomain(username)
	roleName = generator.ToDNS1123Subdomain(roleName)

	prefix := fmt.Sprintf("%s-%s-", username, roleName)

	if MaxGeneratedNameLength-len(prefix) < 0 {
		// If the prefix is too long, use the maximum length available
		extraCharLength := 2 // the format adds 2 dashes
		username, roleName = generator.ToMaxLength(username, roleName, MaxGeneratedNameLength-extraCharLength)
		prefix = fmt.Sprintf("%s-%s-", username, roleName)
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
