package backend_test

import (
	// "context"
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	"github.com/argoproj-labs/ephemeral-access/test/utils"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	accessRequestKind = "AccessRequest"
)

// TestK8sPersister This is an integration test and requires EnvTest to be
// available and properly configured. Run `make setup-envtest` to automatically
// download and configure envtest in the bin/k8s folder in this repo. Alternatively
// run `make test` to run all tests available in this repo.
func TestK8sPersister(t *testing.T) {

	// Setup EnvTest
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	accessRequestCRDPath := filepath.Join("..", "..", "config", "crd", "bases")
	argoprojCRDPath := filepath.Join("..", "..", "test", "manifests", "argoproj", "crd", "schema")
	envTestFolder := fmt.Sprintf("1.30.0-%s-%s", runtime.GOOS, runtime.GOARCH)
	k8sPath := filepath.Join("..", "..", "bin", "k8s", envTestFolder)
	t.Setenv("KUBEBUILDER_ASSETS", k8sPath)
	envTest := &envtest.Environment{
		CRDDirectoryPaths:     []string{accessRequestCRDPath, argoprojCRDPath},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s", envTestFolder),
	}

	// Initialize envTest process
	var err error
	restConfig, err := envTest.Start()
	assert.NoError(t, err)
	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	assert.NoError(t, err)
	defer envTest.Stop()

	// Initialize the backend persister
	logger := log.NewFake()
	p, err := backend.NewK8sPersister(restConfig, logger)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	go func() {
		err := p.StartCache(ctx)
		assert.NoError(t, err)
	}()

	t.Run("will retrieve AccessRequest successfully", func(t *testing.T) {
		// Given
		arName := "some-ar"
		nsName := "retrieve-ar-success"
		username := "some-user"
		appName := "some-app"
		roleName := "some-role"

		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		assert.NoError(t, err)

		ar := utils.NewAccessRequest(arName, nsName, appName, roleName, username)
		err = k8sClient.Create(ctx, ar)
		assert.NoError(t, err)

		// When
		result, err := p.GetAccessRequest(ctx, arName, nsName)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("will return error if ar not found", func(t *testing.T) {
		// Given
		nsName := "retrieve-ar-notfound"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(context.Background(), ns)
		assert.NoError(t, err)

		// When
		result, err := p.GetAccessRequest(ctx, "NOTFOUND", nsName)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "\"NOTFOUND\" not found")
	})
}
