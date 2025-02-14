package backend

import (
	"context"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	argocd "github.com/argoproj-labs/ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
)

const (
	managerName = "argocd-ephemeral-access-backend"

	accessRequestUsernameField     = "spec.subject.username"
	accessRequestAppNameField      = "spec.application.name"
	accessRequestAppNamespaceField = "spec.application.namespace"

	accessBindingRoleField = "spec.roleTemplateRef.name"
)

// Persister defines the operations to interact with the backend persistent
// layer (e.g. Kubernetes)
type Persister interface {

	// CreateAccessRequest creates a new Access Request object and returns it
	CreateAccessRequest(ctx context.Context, ar *api.AccessRequest) (*api.AccessRequest, error)
	// ListAccessRequests returns all the AccessRequest matching the key criterias
	ListAccessRequests(ctx context.Context, key *AccessRequestKey) (*api.AccessRequestList, error)

	// ListAccessBindings returns all the AccessBindings matching the specified role and namespace
	ListAccessBindings(ctx context.Context, roleName, namespace string) (*api.AccessBindingList, error)

	// ListAllAccessBindings returns all the AccessBindings in the given namespace
	ListAllAccessBindings(ctx context.Context, namespace string) (*api.AccessBindingList, error)

	// GetApplication returns an Unstructured object that represents the Application.
	// An Unstructured object is returned to avoid importing the full object type or losing properties
	// during unmarshalling from the partial typed object.
	GetApplication(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error)

	// GetAppProject return an Unstructured object that represents the AppProject.
	// An Unstructured object is returned to avoid importing the full object type or losing properties
	// during unmarshalling from the partial typed object.
	GetAppProject(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error)
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

	err = argocd.AddToScheme(scheme.Scheme)
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

	watchErrorHandler := newWatchErrorHander(logger)

	cacheOpts := cache.Options{
		HTTPClient:                  httpClient,
		Scheme:                      scheme.Scheme,
		Mapper:                      mapper,
		ReaderFailOnMissingInformer: true,
		DefaultWatchErrorHandler:    watchErrorHandler,
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
			Reader:       cache,
			Unstructured: false,
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

func isExpiredError(err error) bool {
	return apierrors.IsResourceExpired(err) || apierrors.IsGone(err)
}

func newWatchErrorHander(logger log.Logger) toolscache.WatchErrorHandler {
	return func(r *toolscache.Reflector, err error) {
		switch {
		case isExpiredError(err):
			logger.Error(err, "Cache watch closed: expired")
		case err == io.EOF:
			logger.Debug("Cache watch closed")
		case err == io.ErrUnexpectedEOF:
			logger.Error(err, "Cache watch closed with unexpected EOF")
		default:
			logger.Error(err, "Cache failed to watch")
		}
	}
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

func (c *K8sPersister) CreateAccessRequest(ctx context.Context, ar *api.AccessRequest) (*api.AccessRequest, error) {
	obj := ar.DeepCopy()
	err := c.client.Create(ctx, obj, &client.CreateOptions{
		FieldManager: managerName,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating access request: %w", err)
	}
	return obj, nil
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
	return c.listAccessBindings(ctx, namespace, selector)
}

func (c *K8sPersister) ListAllAccessBindings(ctx context.Context, namespace string) (*api.AccessBindingList, error) {
	return c.listAccessBindings(ctx, namespace, nil)
}

func (c *K8sPersister) listAccessBindings(ctx context.Context, namespace string, selector fields.Selector) (*api.AccessBindingList, error) {
	list := &api.AccessBindingList{}
	err := c.client.List(ctx, list, &client.ListOptions{Namespace: namespace, FieldSelector: selector})
	if err != nil {
		var selectorStr string
		if selector == nil {
			selectorStr = "nil"
		} else {
			selectorStr = selector.String()
		}
		return nil, fmt.Errorf("error listing access bindings from k8s in namespace %s (selector: %s)  : %w", namespace, selectorStr, err)

	}
	return list, nil
}

func (c *K8sPersister) GetApplication(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(argocd.ApplicationGroupVersionKind)
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	err := c.client.Get(ctx, key, obj)
	if err != nil {
		return nil, fmt.Errorf("error retrieving application %s/%s from k8s: %w", namespace, name, err)
	}
	return obj, nil
}

func (c *K8sPersister) GetAppProject(ctx context.Context, name, namespace string) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(argocd.AppProjectGroupVersionKind)
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	err := c.client.Get(ctx, key, obj)
	if err != nil {
		return nil, fmt.Errorf("error retrieving appproject %s/%s from k8s: %w", namespace, name, err)
	}
	return obj, nil
}
