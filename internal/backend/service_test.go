package backend_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/argoproj-labs/ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/argoproj-labs/ephemeral-access/test/mocks"
	"github.com/argoproj-labs/ephemeral-access/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ControllerNamespace   = "test-controller-ns"
	AccessRequestDuration = time.Minute
)

type serviceFixture struct {
	persister *mocks.MockPersister
	logger    *mocks.MockLogger
	svc       backend.Service
}

func serviceSetup(t *testing.T) *serviceFixture {
	persister := mocks.NewMockPersister(t)
	logger := mocks.NewMockLogger(t)
	logger.EXPECT().Debug(mock.Anything, mock.Anything).Maybe()
	logger.EXPECT().Info(mock.Anything, mock.Anything).Maybe()
	svc := backend.NewDefaultService(persister, logger, ControllerNamespace, AccessRequestDuration)
	return &serviceFixture{
		persister: persister,
		logger:    logger,
		svc:       svc,
	}
}

func TestServiceCreateAccessRequest(t *testing.T) {
	t.Run("will create access request successfully", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		ab := newDefaultAccessBinding()
		f.persister.EXPECT().CreateAccessRequest(mock.Anything, mock.Anything).
			RunAndReturn(func(ctx context.Context, ar *api.AccessRequest) (*api.AccessRequest, error) {
				return ar, nil
			})

		// When
		result, err := f.svc.CreateAccessRequest(context.Background(), key, ab)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "todo-", result.GetGenerateName())
		assert.Equal(t, key.Namespace, result.GetNamespace())
		assert.Equal(t, key.ApplicationName, result.Spec.Application.Name)
		assert.Equal(t, key.ApplicationNamespace, result.Spec.Application.Namespace)
		assert.Equal(t, key.Username, result.Spec.Subject.Username)
		assert.Equal(t, ab.Spec.FriendlyName, result.Spec.Role.FriendlyName)
		assert.Equal(t, ab.Spec.Ordinal, result.Spec.Role.Ordinal)
		assert.Equal(t, ab.Spec.RoleTemplateRef.Name, result.Spec.Role.TemplateName)
		assert.Equal(t, AccessRequestDuration, result.Spec.Duration.Duration)
	})
	t.Run("will return error if k8s request fails", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		ab := newDefaultAccessBinding()
		f.persister.EXPECT().CreateAccessRequest(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("some internal error"))

		// When
		result, err := f.svc.CreateAccessRequest(context.Background(), key, ab)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "some internal error")
	})
}

func TestServiceListAccessRequest(t *testing.T) {
	t.Run("will return access request successfully", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		ar := newAccessRequest(key, "some-role")
		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).Return(&api.AccessRequestList{Items: []api.AccessRequest{*ar}}, nil)

		// When
		result, err := f.svc.ListAccessRequests(context.Background(), key, false)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 1, len(result))
		assert.Equal(t, ar, result[0])
	})
	t.Run("will return error if k8s request fails", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).Return(nil, fmt.Errorf("some internal error"))

		// When
		result, err := f.svc.ListAccessRequests(context.Background(), key, false)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "some internal error")
	})
	t.Run("will filter expired access request", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		ar := newAccessRequest(key, "some-role")
		ar2 := newAccessRequest(key, "some-role")
		utils.ToRequestedState()(ar2)
		utils.ToGrantedState()(ar2)
		utils.ToExpiredState()(ar2)
		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).Return(&api.AccessRequestList{Items: []api.AccessRequest{*ar, *ar2}}, nil)

		// When
		result, err := f.svc.ListAccessRequests(context.Background(), key, false)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 1, len(result))
		assert.Equal(t, ar, result[0])
	})
	t.Run("will sort access request", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		ar := newAccessRequest(key, "some-role")
		ar2 := newAccessRequest(key, "some-role")
		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).Return(&api.AccessRequestList{Items: []api.AccessRequest{*ar2, *ar}}, nil)

		// When
		result, err := f.svc.ListAccessRequests(context.Background(), key, false)
		resultSorted, errSorted := f.svc.ListAccessRequests(context.Background(), key, true)

		// Then
		assert.NoError(t, err)
		assert.NoError(t, errSorted)
		assert.NotNil(t, result)
		assert.NotNil(t, resultSorted)
		assert.Equal(t, 2, len(result))
		assert.Equal(t, 2, len(resultSorted))
		assert.Equal(t, ar, result[1])
		assert.Equal(t, ar2, result[0])
		assert.Equal(t, ar, resultSorted[0])
		assert.Equal(t, ar2, resultSorted[1])
	})
}

func TestServiceGetAccessRequestByRole(t *testing.T) {
	t.Run("will return most important access request matching role", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		roleName := "some-role"
		ar := newAccessRequest(key, roleName)
		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).Return(&api.AccessRequestList{Items: []api.AccessRequest{*ar}}, nil)

		// When
		result, err := f.svc.GetAccessRequestByRole(context.Background(), key, roleName)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, ar.GetName(), result.GetName())
		assert.Equal(t, ar.GetNamespace(), result.GetNamespace())
	})
	t.Run("will return nil if no access request match role", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		roleName := "some-role"
		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).Return(&api.AccessRequestList{}, nil)

		// When
		ar, err := f.svc.GetAccessRequestByRole(context.Background(), key, roleName)

		// Then
		assert.NoError(t, err)
		assert.Nil(t, ar)
	})
	t.Run("will return error if k8s request fails", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "namespace",
			ApplicationName:      "appName",
			ApplicationNamespace: "appNs",
			Username:             "username",
		}
		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).
			Return(nil, fmt.Errorf("some internal error"))
		// When
		result, err := f.svc.ListAccessRequests(context.Background(), key, false)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "some internal error")
	})
}

func TestServiceGetGrantingAccessBinding(t *testing.T) {
	t.Run("will return binding when granting in target namespace", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		app := &unstructured.Unstructured{}
		project := &unstructured.Unstructured{}
		roleName := "some-role"
		namespace := "some-namespace"
		subject := "my-subject"
		groups := []string{subject}
		ab := newAccessBinding(namespace, roleName, subject)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, namespace).Return(&api.AccessBindingList{Items: []api.AccessBinding{*ab}}, nil)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, ControllerNamespace).Return(&api.AccessBindingList{}, nil)

		// When
		result, err := f.svc.GetGrantingAccessBinding(context.Background(), roleName, namespace, groups, app, project)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, ab, result)
	})
	t.Run("will return binding when granting in controller namespace", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		app := &unstructured.Unstructured{}
		project := &unstructured.Unstructured{}
		roleName := "some-role"
		namespace := "some-namespace"
		subject := "my-subject"
		groups := []string{subject}
		ab := newAccessBinding(namespace, roleName, subject)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, namespace).Return(&api.AccessBindingList{}, nil)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, ControllerNamespace).Return(&api.AccessBindingList{Items: []api.AccessBinding{*ab}}, nil)

		// When
		result, err := f.svc.GetGrantingAccessBinding(context.Background(), roleName, namespace, groups, app, project)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, ab, result)
	})
	t.Run("will prioritize access binding from target namespace", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		app := &unstructured.Unstructured{}
		project := &unstructured.Unstructured{}
		roleName := "some-role"
		namespace := "some-namespace"
		subject := "my-subject"
		groups := []string{subject}
		ab := newAccessBinding(namespace, roleName, subject)
		ab2 := newAccessBinding(namespace, roleName, subject)
		ab2.Name = "controller-binding"
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, namespace).Return(&api.AccessBindingList{Items: []api.AccessBinding{*ab}}, nil)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, ControllerNamespace).Return(&api.AccessBindingList{Items: []api.AccessBinding{*ab2}}, nil)

		// When
		result, err := f.svc.GetGrantingAccessBinding(context.Background(), roleName, namespace, groups, app, project)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, ab, result)
	})
	t.Run("will return error if k8s request fails for target namespace", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		app := &unstructured.Unstructured{}
		project := &unstructured.Unstructured{}
		roleName := "some-role"
		namespace := "some-namespace"
		subject := "my-subject"
		groups := []string{subject}
		ab := newAccessBinding(namespace, roleName, subject)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, namespace).Return(nil, fmt.Errorf("some internal error"))
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, ControllerNamespace).Return(&api.AccessBindingList{Items: []api.AccessBinding{*ab}}, nil).Maybe()

		// When
		result, err := f.svc.GetGrantingAccessBinding(context.Background(), roleName, namespace, groups, app, project)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "some internal error")
	})
	t.Run("will return error if k8s request fails for controller namespace", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		app := &unstructured.Unstructured{}
		project := &unstructured.Unstructured{}
		roleName := "some-role"
		namespace := "some-namespace"
		subject := "my-subject"
		groups := []string{subject}
		ab := newAccessBinding(namespace, roleName, subject)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, namespace).Return(&api.AccessBindingList{Items: []api.AccessBinding{*ab}}, nil).Maybe()
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, ControllerNamespace).Return(nil, fmt.Errorf("some internal error"))

		// When
		result, err := f.svc.GetGrantingAccessBinding(context.Background(), roleName, namespace, groups, app, project)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "some internal error")
	})
	t.Run("will return nil if no bindings are found", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		app := &unstructured.Unstructured{}
		project := &unstructured.Unstructured{}
		roleName := "some-role"
		namespace := "some-namespace"
		subject := "my-subject"
		groups := []string{subject}
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, namespace).Return(&api.AccessBindingList{}, nil)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, ControllerNamespace).Return(&api.AccessBindingList{}, nil)

		// When
		result, err := f.svc.GetGrantingAccessBinding(context.Background(), roleName, namespace, groups, app, project)

		// Then
		assert.NoError(t, err)
		assert.Nil(t, result)
	})
	t.Run("will return nil if no bindings are granting", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		app := &unstructured.Unstructured{}
		project := &unstructured.Unstructured{}
		roleName := "some-role"
		namespace := "some-namespace"
		subject := "my-subject"
		groups := []string{"another-subject-that-does-not-match"}
		ab := newAccessBinding(namespace, roleName, subject)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, namespace).Return(&api.AccessBindingList{Items: []api.AccessBinding{*ab}}, nil)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, ControllerNamespace).Return(&api.AccessBindingList{}, nil)

		// When
		result, err := f.svc.GetGrantingAccessBinding(context.Background(), roleName, namespace, groups, app, project)

		// Then
		assert.NoError(t, err)
		assert.Nil(t, result)
	})
	t.Run("will return binding when the go template match", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		app, err := utils.ToUnstructured(&v1alpha1.Application{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					"hello": "world",
				},
			},
		})
		require.NoError(t, err)
		project, err := utils.ToUnstructured(&v1alpha1.AppProject{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					"foo": "bar",
				},
			},
		})
		require.NoError(t, err)
		roleName := "some-role"
		namespace := "some-namespace"
		subject := `group:{{ index .app.metadata.annotations "hello" }}[{{ index .project.metadata.annotations "foo" }}]`
		groups := []string{"group:world[bar]"}
		ab := newAccessBinding(namespace, roleName, subject)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, namespace).Return(&api.AccessBindingList{Items: []api.AccessBinding{*ab}}, nil)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, ControllerNamespace).Return(&api.AccessBindingList{}, nil)

		// When
		result, err := f.svc.GetGrantingAccessBinding(context.Background(), roleName, namespace, groups, app, project)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, ab, result)
	})
	t.Run("will not fail if the binding template is invalid", func(t *testing.T) {
		// Given
		var errorMsg string
		f := serviceSetup(t)
		app := &unstructured.Unstructured{}
		project := &unstructured.Unstructured{}
		roleName := "some-role"
		namespace := "some-namespace"
		subject := "{{ invalid go template }}"
		groups := []string{"some group"}
		ab := newAccessBinding(namespace, roleName, subject)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, namespace).Return(&api.AccessBindingList{Items: []api.AccessBinding{*ab}}, nil)
		f.persister.EXPECT().ListAccessBindings(mock.Anything, roleName, ControllerNamespace).Return(&api.AccessBindingList{}, nil)
		f.logger.EXPECT().Error(mock.Anything, mock.Anything).Run(func(err error, msg string, keysAndValues ...interface{}) {
			errorMsg = msg
		}).Once()

		// When
		result, err := f.svc.GetGrantingAccessBinding(context.Background(), roleName, namespace, groups, app, project)

		// Then
		assert.NoError(t, err)
		assert.Nil(t, result)
		assert.Contains(t, errorMsg, "cannot render subjects")
	})
}

func Test_defaultAccessRequestSort(t *testing.T) {

	t.Run("equals on object equality", func(t *testing.T) {
		// Given
		a := utils.NewAccessRequestCreated()
		b := a.DeepCopy()

		// When
		got := backend.DefaultAccessRequestSort(a, b)

		// Then
		require.Equal(t, 0, got)
	})

	t.Run("array should be ordered by status", func(t *testing.T) {
		// Given
		created := utils.NewAccessRequestCreated()
		requested := created.DeepCopy()
		utils.ToRequestedState()(requested)

		invalid := created.DeepCopy()
		utils.ToInvalidState()(invalid)

		granted := requested.DeepCopy()
		utils.ToGrantedState()(granted)

		denied := requested.DeepCopy()
		utils.ToDeniedState()(denied)

		expired := granted.DeepCopy()
		utils.ToExpiredState()(expired)

		items := []*api.AccessRequest{
			expired,
			denied,
			granted,
			created,
			requested,
			invalid,
		}

		// When
		slices.SortStableFunc(items, backend.DefaultAccessRequestSort)

		// Then
		require.Equal(t, 6, len(items))
		// compare each object because it makes it easier to investigate on test failures
		require.Equal(t, created, items[0])
		require.Equal(t, requested, items[1])
		require.Equal(t, granted, items[2])
		require.Equal(t, denied, items[3])
		require.Equal(t, invalid, items[4])
		require.Equal(t, expired, items[5])
	})
	t.Run("array should be ordered by role ordinal for the same status", func(t *testing.T) {
		// Given
		base := utils.NewAccessRequestCreated(utils.WithRole())
		first := base.DeepCopy()
		first.Spec.Role.Ordinal = 0
		second := base.DeepCopy()
		second.Spec.Role.Ordinal = 1
		third := base.DeepCopy()
		third.Spec.Role.Ordinal = 2

		items := []*api.AccessRequest{
			third,
			first,
			second,
		}

		// When
		slices.SortStableFunc(items, backend.DefaultAccessRequestSort)

		// Then
		require.Equal(t, 3, len(items))
		// compare each object because it makes it easier to investigate on test failures
		require.Equal(t, first, items[0])
		require.Equal(t, second, items[1])
		require.Equal(t, third, items[2])
	})
	t.Run("array should be ordered by role name for the same status and ordinal", func(t *testing.T) {
		// Given
		base := utils.NewAccessRequestCreated(utils.WithRole())
		first := base.DeepCopy()
		first.Spec.Role.TemplateName = "a"
		second := base.DeepCopy()
		second.Spec.Role.TemplateName = "b"
		third := base.DeepCopy()
		third.Spec.Role.TemplateName = "c"

		items := []*api.AccessRequest{
			third,
			first,
			second,
		}

		// When
		slices.SortStableFunc(items, backend.DefaultAccessRequestSort)

		// Then
		require.Equal(t, 3, len(items))
		// compare each object because it makes it easier to investigate on test failures
		require.Equal(t, first, items[0])
		require.Equal(t, second, items[1])
		require.Equal(t, third, items[2])
	})
	t.Run("array should be ordered by creation for the same status, ordinal and role name", func(t *testing.T) {
		// Given
		base := utils.NewAccessRequestCreated(utils.WithRole())
		first := base.DeepCopy()
		first.CreationTimestamp = v1.NewTime(v1.Now().Add(time.Second * 1))
		second := base.DeepCopy()
		second.CreationTimestamp = v1.NewTime(v1.Now().Add(time.Second * 2))
		third := base.DeepCopy()
		third.CreationTimestamp = v1.NewTime(v1.Now().Add(time.Second * 3))

		items := []*api.AccessRequest{
			third,
			first,
			second,
		}

		// When
		slices.SortStableFunc(items, backend.DefaultAccessRequestSort)

		// Then
		require.Equal(t, 3, len(items))
		// compare each object because it makes it easier to investigate on test failures
		require.Equal(t, first, items[0])
		require.Equal(t, second, items[1])
		require.Equal(t, third, items[2])
	})
}
