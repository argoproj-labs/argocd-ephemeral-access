package backend_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/argoproj-labs/ephemeral-access/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	tests := []struct {
		name     string
		a        *api.AccessRequest
		b        *api.AccessRequest
		expected int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := backend.DefaultAccessRequestSort(tt.a, tt.b); got != tt.expected {
				t.Errorf("defaultAccessRequestSort() = %v, want %v", got, tt.expected)
			}
		})
	}
}
