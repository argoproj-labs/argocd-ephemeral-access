package backend_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/argoproj-labs/ephemeral-access/test/mocks"
	"github.com/argoproj-labs/ephemeral-access/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ControllerNamespace = "test-controller-ns"
)

type serviceFixture struct {
	persister *mocks.MockPersister
	logger    *mocks.MockLogger
	svc       backend.Service
}

func serviceSetup(t *testing.T) *serviceFixture {
	persister := mocks.NewMockPersister(t)
	logger := mocks.NewMockLogger(t)
	svc := backend.NewDefaultService(persister, logger, ControllerNamespace)
	return &serviceFixture{
		persister: persister,
		logger:    logger,
		svc:       svc,
	}
}

func TestServiceCreateAccessRequest(t *testing.T) {
	t.Run("will create access request successfully", func(t *testing.T) {
	})
	t.Run("will return error if k8s request fails", func(t *testing.T) {
	})
}

func TestServiceListAccessRequest(t *testing.T) {
	t.Run("will return access request successfully", func(t *testing.T) {
	})
	t.Run("will return error if k8s request fails", func(t *testing.T) {
	})
	t.Run("will filter expired access request", func(t *testing.T) {
	})
	t.Run("will sort access request", func(t *testing.T) {
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
		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).Return(&v1alpha1.AccessRequestList{Items: []v1alpha1.AccessRequest{*ar}}, nil)

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
		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).Return(&v1alpha1.AccessRequestList{}, nil)

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
	t.Run("will not return binding when granting", func(t *testing.T) {
	})
	t.Run("will get access binding from target namespace", func(t *testing.T) {
	})
	t.Run("will get access binding from controller namespace", func(t *testing.T) {
	})
	t.Run("will prioritize access binding from target namespace", func(t *testing.T) {
	})
	t.Run("will return error if k8s request fails", func(t *testing.T) {
	})
	t.Run("will return nil if no bindings are found", func(t *testing.T) {
	})
	t.Run("will return nil if no bindings are granting", func(t *testing.T) {
	})
	t.Run("will not fail if the binding template is invalid", func(t *testing.T) {
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
		}

		// When
		slices.SortStableFunc(items, backend.DefaultAccessRequestSort)

		// Then
		require.Equal(t, 5, len(items))
		// compare each object because it makes it easier to investigate on test failures
		require.Equal(t, created, items[0])
		require.Equal(t, requested, items[1])
		require.Equal(t, granted, items[2])
		require.Equal(t, denied, items[3])
		require.Equal(t, expired, items[4])
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
