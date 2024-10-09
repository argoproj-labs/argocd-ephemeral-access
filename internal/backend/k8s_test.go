package backend_test

import (
	// "context"
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/argoproj-labs/ephemeral-access/api/argoproj/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	"github.com/argoproj-labs/ephemeral-access/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	accessRequestKind = "AccessRequest"
)

// eventually runs f until it returns true, an error or the timeout expires
func eventually(f func() (bool, error), timeout time.Duration, interval time.Duration) error {
	start := time.Now()
	for {
		if ok, err := f(); ok {
			return nil
		} else if err != nil {
			return err
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("timed out waiting for eventual success")
		}
		time.Sleep(interval)
	}
}

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
	require.NoError(t, err)
	k8sClient, err := client.New(restConfig, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)
	defer envTest.Stop()

	// Initialize the backend persister
	logger := log.NewFake()
	p, err := backend.NewK8sPersister(restConfig, logger)
	require.NoError(t, err)
	require.NotNil(t, p)
	go func() {
		err := p.StartCache(ctx)
		require.NoError(t, err)
	}()

	t.Run("will create AccessRequest successfully", func(t *testing.T) {
		// Given
		nsName := "create-ar-success"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		require.NoError(t, err)

		ar := utils.NewAccessRequestCreated()
		ar.ObjectMeta.Namespace = nsName

		// When
		result, err := p.CreateAccessRequest(ctx, ar)

		// Then
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEqual(t, ar, result)
		assert.Equal(t, ar.GetName(), result.GetName())
		assert.Equal(t, ar.GetNamespace(), result.GetNamespace())
	})

	t.Run("will return an error if create fails", func(t *testing.T) {
		// Given
		nsName := "create-ar-error"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		require.NoError(t, err)

		ar := utils.NewAccessRequestCreated()
		ar.ObjectMeta.Namespace = nsName
		ar.ObjectMeta.Name = "--invalid--"

		// When
		result, err := p.CreateAccessRequest(ctx, ar)

		// Then
		assert.Error(t, err)
		assert.ErrorContains(t, err, "metadata.name: Invalid value")
		assert.Nil(t, result)
	})

	t.Run("will list AccessRequest successfully", func(t *testing.T) {
		// Given
		nsName := "list-ar-success"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		require.NoError(t, err)

		roleName := "some-role"
		key := &backend.AccessRequestKey{
			Namespace:            nsName,
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}

		ar := newAccessRequest(key, roleName)
		err = k8sClient.Create(ctx, ar)
		require.NoError(t, err)

		// When
		expectedItems := 1
		eventually(func() (bool, error) {
			result, err := p.ListAccessRequests(ctx, key)
			return result != nil && len(result.Items) == expectedItems, err
		}, 5*time.Second, time.Second)
		result, err := p.ListAccessRequests(ctx, key)

		// Then
		assert.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, expectedItems, len(result.Items))
		assert.Equal(t, ar.GetName(), result.Items[0].Name)
		assert.Equal(t, ar.GetNamespace(), result.Items[0].Namespace)
		assert.Equal(t, ar.Spec.Application.Name, result.Items[0].Spec.Application.Name)
		assert.Equal(t, ar.Spec.Application.Namespace, result.Items[0].Spec.Application.Namespace)
		assert.Equal(t, ar.Spec.Subject.Username, result.Items[0].Spec.Subject.Username)
	})

	t.Run("will only list AccessRequest matching filters", func(t *testing.T) {
		// Given
		nsName := "list-ar-filtered"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		require.NoError(t, err)

		otherNsName := nsName + "-other"
		otherNs := utils.NewNamespace(otherNsName)
		err = k8sClient.Create(ctx, otherNs)
		require.NoError(t, err)

		roleName := "some-role"
		key := &backend.AccessRequestKey{
			Namespace:            nsName,
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		ar := newAccessRequest(key, roleName)
		ar.ObjectMeta.Name = "ar-expected"
		err = k8sClient.Create(ctx, ar)
		require.NoError(t, err)

		anotherNamespaceKey := &backend.AccessRequestKey{
			Namespace:            otherNsName,
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		arNs := newAccessRequest(anotherNamespaceKey, roleName)
		arNs.ObjectMeta.Name = "ar-ns"
		err = k8sClient.Create(ctx, arNs)
		require.NoError(t, err)

		anotherApp := &backend.AccessRequestKey{
			Namespace:            nsName,
			ApplicationName:      "another-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		arApp := newAccessRequest(anotherApp, roleName)
		arApp.ObjectMeta.Name = "ar-app"
		err = k8sClient.Create(ctx, arApp)
		require.NoError(t, err)

		anotherAppNamespaceKey := &backend.AccessRequestKey{
			Namespace:            nsName,
			ApplicationName:      "some-app",
			ApplicationNamespace: "another-app-ns",
			Username:             "some-user",
		}
		arAppNs := newAccessRequest(anotherAppNamespaceKey, roleName)
		arAppNs.ObjectMeta.Name = "ar-app-ns"
		err = k8sClient.Create(ctx, arAppNs)
		require.NoError(t, err)

		anotherUserKey := &backend.AccessRequestKey{
			Namespace:            nsName,
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "another-user",
		}
		arUser := newAccessRequest(anotherUserKey, roleName)
		arUser.ObjectMeta.Name = "ar-user"
		err = k8sClient.Create(ctx, arUser)
		require.NoError(t, err)

		// When
		expectedItems := 1
		eventually(func() (bool, error) {
			result, err := p.ListAccessRequests(ctx, key)
			return result != nil && len(result.Items) == expectedItems, err
		}, 5*time.Second, time.Second)
		result, err := p.ListAccessRequests(ctx, key)

		// Then
		assert.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, expectedItems, len(result.Items))
		assert.Equal(t, ar.GetName(), result.Items[0].Name)
		assert.Equal(t, ar.GetNamespace(), result.Items[0].Namespace)
		assert.Equal(t, ar.Spec.Application.Name, result.Items[0].Spec.Application.Name)
		assert.Equal(t, ar.Spec.Application.Namespace, result.Items[0].Spec.Application.Namespace)
		assert.Equal(t, ar.Spec.Subject.Username, result.Items[0].Spec.Subject.Username)
	})

	t.Run("will return empty if no ar are found", func(t *testing.T) {
		// Given
		nsName := "list-ar-notfound"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		require.NoError(t, err)

		key := &backend.AccessRequestKey{
			Namespace:            nsName,
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}

		// When
		result, err := p.ListAccessRequests(ctx, key)

		// Then
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 0, len(result.Items))
	})

	t.Run("will list AccessBindings successfully", func(t *testing.T) {
		// Given
		nsName := "list-ab-success"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		require.NoError(t, err)

		roleName := "some-role"

		ab := newDefaultAccessBinding()
		ab.ObjectMeta.Namespace = nsName
		ab.ObjectMeta.Name = "ab-expected"
		ab.Spec.RoleTemplateRef.Name = roleName
		err = k8sClient.Create(ctx, ab)
		require.NoError(t, err)

		// When
		expectedItems := 1
		eventually(func() (bool, error) {
			result, err := p.ListAccessBindings(ctx, roleName, nsName)
			return result != nil && len(result.Items) == expectedItems, err
		}, 5*time.Second, time.Second)
		result, err := p.ListAccessBindings(ctx, roleName, nsName)

		// Then
		assert.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, expectedItems, len(result.Items))
		assert.Equal(t, ab.GetName(), result.Items[0].Name)
		assert.Equal(t, ab.GetNamespace(), result.Items[0].Namespace)
	})

	t.Run("will only list AccessBindings matching filters", func(t *testing.T) {
		// Given
		nsName := "list-ab-filtered"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		require.NoError(t, err)

		otherNsName := nsName + "-other"
		otherNs := utils.NewNamespace(otherNsName)
		err = k8sClient.Create(ctx, otherNs)
		require.NoError(t, err)

		roleName := "some-role"

		ab := newDefaultAccessBinding()
		ab.ObjectMeta.Namespace = nsName
		ab.ObjectMeta.Name = "ab-expected"
		ab.Spec.RoleTemplateRef.Name = roleName
		err = k8sClient.Create(ctx, ab)
		require.NoError(t, err)

		abNs := newDefaultAccessBinding()
		abNs.ObjectMeta.Namespace = otherNsName
		abNs.ObjectMeta.Name = "ab-other-ns"
		abNs.Spec.RoleTemplateRef.Name = roleName
		err = k8sClient.Create(ctx, abNs)
		require.NoError(t, err)

		abRole := newDefaultAccessBinding()
		abRole.ObjectMeta.Namespace = nsName
		abRole.ObjectMeta.Name = "ab-other-role"
		abRole.Spec.RoleTemplateRef.Name = "other-role"
		err = k8sClient.Create(ctx, abRole)
		require.NoError(t, err)

		// When
		expectedItems := 1
		eventually(func() (bool, error) {
			result, err := p.ListAccessBindings(ctx, roleName, nsName)
			return result != nil && len(result.Items) == expectedItems, err
		}, 5*time.Second, time.Second)
		result, err := p.ListAccessBindings(ctx, roleName, nsName)

		// Then
		assert.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, expectedItems, len(result.Items))
		assert.Equal(t, ab.GetName(), result.Items[0].Name)
		assert.Equal(t, ab.GetNamespace(), result.Items[0].Namespace)
	})

	t.Run("will return empty if no AccessBindings are found", func(t *testing.T) {
		// Given
		nsName := "list-ab-notfound"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		require.NoError(t, err)

		// When
		result, err := p.ListAccessBindings(ctx, "", nsName)

		// Then
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 0, len(result.Items))
	})

	t.Run("will successfully get the Application", func(t *testing.T) {
		// Given
		nsName := "get-app"
		name := "my-app"
		destName := "dest-name-value"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		require.NoError(t, err)

		app := &v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Namespace: nsName,
				Name:      name,
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "test",
			},
		}
		app.SetGroupVersionKind(v1alpha1.ApplicationGroupVersionKind)
		appU, err := utils.ToUnstructured(app)
		require.NoError(t, err)
		// spec.destination is required, but not defined in the ephemeral-access-spec
		require.NoError(t, unstructured.SetNestedField(appU.Object, destName, "spec", "destination", "name"))
		err = k8sClient.Create(ctx, appU)
		require.NoError(t, err)

		// When
		result, err := p.GetApplication(ctx, name, nsName)

		// Then
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, name, result.GetName())
		assert.Equal(t, nsName, result.GetNamespace())
		gotDestName, ok, err := unstructured.NestedString(result.Object, "spec", "destination", "name")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, destName, gotDestName)
	})

	t.Run("will return an error if Application does not exist", func(t *testing.T) {
		// Given
		nsName := "get-app-notfound"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		require.NoError(t, err)

		// When
		result, err := p.GetApplication(ctx, "not-found", nsName)

		// Then
		assert.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
		assert.Nil(t, result)
	})

	t.Run("will successfully get the AppProject", func(t *testing.T) {
		// Given
		nsName := "get-project"
		name := "my-project"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		require.NoError(t, err)

		project := &v1alpha1.AppProject{
			ObjectMeta: v1.ObjectMeta{
				Namespace: nsName,
				Name:      name,
			},
			Spec: v1alpha1.AppProjectSpec{
				Roles: []v1alpha1.ProjectRole{
					{Name: "test"},
				},
			},
		}
		err = k8sClient.Create(ctx, project)
		require.NoError(t, err)

		// When
		result, err := p.GetAppProject(ctx, name, nsName)

		// Then
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, name, result.GetName())
		assert.Equal(t, nsName, result.GetNamespace())
		roles, ok, err := unstructured.NestedSlice(result.Object, "spec", "roles")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, len(project.Spec.Roles), len(roles))
	})

	t.Run("will return an error if AppProject does not exist", func(t *testing.T) {
		// Given
		nsName := "get-project-notfound"
		ns := utils.NewNamespace(nsName)
		err = k8sClient.Create(ctx, ns)
		require.NoError(t, err)

		// When
		result, err := p.GetAppProject(ctx, "not-found", nsName)

		// Then
		assert.Error(t, err)
		assert.True(t, apierrors.IsNotFound(err))
		assert.Nil(t, result)
	})

}
