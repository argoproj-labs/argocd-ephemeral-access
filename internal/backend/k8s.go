package backend

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	appprojectv1alpha1 "github.com/argoproj-labs/ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	ephemeralaccessv1alpha1 "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
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
	client client.Client
	cache  cache.Cache
}

// NewK8sPersister will return a new K8sPersister instance.
func NewK8sPersister(config *rest.Config) (*K8sPersister, error) {
	err := ephemeralaccessv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, fmt.Errorf("error adding ephemeralaccessv1alpha1 to k8s scheme: %w", err)
	}

	err = appprojectv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, fmt.Errorf("error adding appprojectv1alpha1 to k8s scheme: %w", err)
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

	clientOpts := client.Options{
		HTTPClient: httpClient,
		Scheme:     scheme.Scheme,
		Mapper:     mapper,
		Cache: &client.CacheOptions{
			Reader: cache,
		},
	}
	k8sClient, err := client.New(config, clientOpts)
	return &K8sPersister{
		client: k8sClient,
		cache:  cache,
	}, nil
}

// StartCache will initialize the Kubernetes persister cache and block the call.
func (p *K8sPersister) StartCache(ctx context.Context) error {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle graceful shutdown.
	quit := make(chan os.Signal, 1)
	defer close(quit)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

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
		return nil
	case err := <-errCh:
		return err
	case <-quit:
		fmt.Println("Shutting down k8s cache...")
		return nil
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
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
	ar := &api.AccessRequest{}
	err := c.client.Get(ctx, key, ar)
	if err != nil {
		return nil, fmt.Errorf("error retrieving accessrequest %s/%s from k8s: %w", namespace, name, err)
	}
	return ar, nil
}
