package backend

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	argoprojv1alpha1 "github.com/argoproj-labs/ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
)

const (
	resourceType = "accessrequests"

	accessRequestUsernameField     = "spec.subject.username"
	accessRequestAppNameField      = "spec.application.name"
	accessRequestAppNamespaceField = "spec.application.namespace"

	accessBindingRoleField = "spec.roleTemplateRef.name"
)

// Persister defines the operations to interact with the backend persistent
// layer (e.g. Kubernetes)
type Persister interface {

	// CreateAccessRequest creates a new Access Request object
	CreateAccessRequest(ctx context.Context, ar *api.AccessRequest) (*api.AccessRequest, error)
	// ListAccessRequests returns all the AccessRequest matching the key criterias
	ListAccessRequests(ctx context.Context, key *AccessRequestKey) (*api.AccessRequestList, error)

	// ListAccessRequests returns all the AccessBindings matching the specified role and namespace
	ListAccessBindings(ctx context.Context, roleName, namespace string) (*api.AccessBindingList, error)
}

// K8sPersister is a K8s implementation for the Persister interface.
type K8sPersister struct {
	client client.Client
	cache  cache.Cache
	logger log.Logger
}

// NewK8sPersister will return a new K8sPersister instance.
func NewK8sPersister(config *rest.Config, logger log.Logger) (*K8sPersister, error) {
	err := api.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, fmt.Errorf("error adding ephemeralaccessv1alpha1 to k8s scheme: %w", err)
	}

	err = argoprojv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, fmt.Errorf("error adding argoprojv1alpha1 to k8s scheme: %w", err)
	}

	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("error creating k8s http client: %w", err)
	}

	mapper, err := apiutil.NewDynamicRESTMapper(config, httpClient)
	if err != nil {
		return nil, fmt.Errorf("error creating rest mapper: %w", err)
	}

	cacheOpts := cache.Options{
		HTTPClient: httpClient,
		Scheme:     scheme.Scheme,
		Mapper:     mapper,
	}
	cache, err := cache.New(config, cacheOpts)
	if err != nil {
		return nil, fmt.Errorf("error creating cluster cache: %w", err)
	}

	err = cache.IndexField(context.Background(), &api.AccessRequest{}, accessRequestUsernameField, func(obj client.Object) []string {
		ar := obj.(*api.AccessRequest)
		if ar.Spec.Subject.Username == "" {
			return nil
		}
		return []string{ar.Spec.Subject.Username}
	})
	if err != nil {
		return nil, fmt.Errorf("error adding AccessRequest index for field %s: %w", accessRequestUsernameField, err)
	}

	err = cache.IndexField(context.Background(), &api.AccessRequest{}, accessRequestAppNamespaceField, func(obj client.Object) []string {
		ar := obj.(*api.AccessRequest)
		if ar.Spec.Application.Namespace == "" {
			return nil
		}
		return []string{ar.Spec.Application.Namespace}
	})
	if err != nil {
		return nil, fmt.Errorf("error adding AccessRequest index for field %s: %w", accessRequestAppNamespaceField, err)
	}

	err = cache.IndexField(context.Background(), &api.AccessRequest{}, accessRequestAppNameField, func(obj client.Object) []string {
		ar := obj.(*api.AccessRequest)
		if ar.Spec.Application.Name == "" {
			return nil
		}
		return []string{ar.Spec.Application.Name}
	})
	if err != nil {
		return nil, fmt.Errorf("error adding AccessRequest index for field %s: %w", accessRequestAppNameField, err)
	}

	err = cache.IndexField(context.Background(), &api.AccessBinding{}, accessBindingRoleField, func(obj client.Object) []string {
		b := obj.(*api.AccessBinding)
		if b.Spec.RoleTemplateRef.Name == "" {
			return nil
		}
		return []string{b.Spec.RoleTemplateRef.Name}
	})
	if err != nil {
		return nil, fmt.Errorf("error adding AccessBinding index for field %s: %w", accessBindingRoleField, err)
	}

	clientOpts := client.Options{
		HTTPClient: httpClient,
		Scheme:     scheme.Scheme,
		Mapper:     mapper,
		Cache: &client.CacheOptions{
			Reader: cache,
		},
	}
	k8sClient, err := client.New(config, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("error creating k8s client: %w", err)
	}

	return &K8sPersister{
		client: k8sClient,
		cache:  cache,
		logger: logger,
	}, nil
}

// StartCache will initialize the Kubernetes persister cache and block the call.
func (p *K8sPersister) StartCache(ctx context.Context) error {
	p.logger.Info("Starting Kubernetes cache...")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error)
	defer close(errCh)
	go func() {
		err := p.cache.Start(ctx)
		if err != nil {
			errCh <- fmt.Errorf("cache start error: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		p.logger.Info("Shutting down Kubernetes cache: Context done")
		return nil
	case err := <-errCh:
		return err
	}
}

// GetAccessRequestResource return a GroupVersionResource schema for the AccessRequest CRD.
func GetAccessRequestResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    api.GroupVersion.Group,
		Version:  api.GroupVersion.Version,
		Resource: resourceType,
	}
}

func (c *K8sPersister) CreateAccessRequest(ctx context.Context, ar *api.AccessRequest) (*api.AccessRequest, error) {
	panic("unimplemented")
}

func (c *K8sPersister) ListAccessRequests(ctx context.Context, key *AccessRequestKey) (*api.AccessRequestList, error) {
	var selector = fields.SelectorFromSet(
		fields.Set{
			accessRequestUsernameField:     key.Username,
			accessRequestAppNameField:      key.ApplicationName,
			accessRequestAppNamespaceField: key.ApplicationNamespace,
		},
	)

	list := &api.AccessRequestList{}
	err := c.client.List(ctx, list, &client.ListOptions{Namespace: key.Namespace, FieldSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("error listing access request for user %s in app %s/%s from k8s: %w", key.Username, key.ApplicationNamespace, key.ApplicationName, err)
	}
	return list, nil
}

func (c *K8sPersister) ListAccessBindings(ctx context.Context, roleName, namespace string) (*api.AccessBindingList, error) {
	var selector = fields.SelectorFromSet(
		fields.Set{
			accessBindingRoleField: roleName,
		},
	)

	list := &api.AccessBindingList{}
	err := c.client.List(ctx, list, &client.ListOptions{Namespace: namespace, FieldSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("error listing access bindings for role %s in namespace %s from k8s: %w", roleName, namespace, err)
	}
	return list, nil
}
