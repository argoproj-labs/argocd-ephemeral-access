package backend_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/argoproj-labs/ephemeral-access/test/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type serviceFixture struct {
	persister *mocks.MockPersister
	logger    *mocks.MockLogger
}

func serviceSetup(t *testing.T) *serviceFixture {
	return &serviceFixture{
		persister: mocks.NewMockPersister(t),
		logger:    mocks.NewMockLogger(t),
	}
}

func TestServiceGetAccessRequest(t *testing.T) {
	t.Run("GetAccessRequest will return access request successfully", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		svc := backend.NewDefaultService(f.persister, f.logger)
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
		result, err := svc.GetAccessRequest(context.Background(), key, roleName)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, ar.GetName(), result.GetName())
		assert.Equal(t, ar.GetNamespace(), result.GetNamespace())
	})
	t.Run("GetAccessRequest will return nil and no error if accessrequest is not found", func(t *testing.T) {
		// Given
		f := setup(t)
		svc := backend.NewDefaultService(f.persister, f.logger)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		roleName := "some-role"
		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).Return(&v1alpha1.AccessRequestList{}, nil)

		// When
		ar, err := svc.GetAccessRequest(context.Background(), key, roleName)

		// Then
		assert.NoError(t, err)
		assert.Nil(t, ar)
	})
	t.Run("ListAccessRequests will return error if k8s request fails", func(t *testing.T) {
		// Given
		f := setup(t)
		svc := backend.NewDefaultService(f.persister, f.logger)
		key := &backend.AccessRequestKey{
			Namespace:            "namespace",
			ApplicationName:      "appName",
			ApplicationNamespace: "appNs",
			Username:             "username",
		}
		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).
			Return(nil, fmt.Errorf("some internal error"))
		// When
		result, err := svc.ListAccessRequests(context.Background(), key, false)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "some internal error")
	})

}
