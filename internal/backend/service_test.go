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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestDefaultService(t *testing.T) {
	type fixture struct {
		persister *mocks.MockPersister
		logger    *mocks.MockLogger
	}
	setup := func(t *testing.T) *fixture {
		return &fixture{
			persister: mocks.NewMockPersister(t),
			logger:    mocks.NewMockLogger(t),
		}
	}
	t.Run("GetAccessRequest will return access request successfully", func(t *testing.T) {
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
	t.Run("GetAccessRequest will return nil and no error if accessrequest not found", func(t *testing.T) {
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
		ar := newAccessRequest(key, roleName)
		r := backend.GetAccessRequestResource()

		gr := schema.GroupResource{
			Group:    r.Group,
			Resource: r.Resource,
		}
		notFoundError := errors.NewNotFound(gr, "some-err")
		f.persister.EXPECT().GetAccessRequest(mock.Anything, ar.GetName(), ar.GetNamespace()).
			Return(nil, notFoundError)

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
