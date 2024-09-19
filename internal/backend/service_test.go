package backend_test

import (
	"context"
	"fmt"
	"testing"

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
	t.Run("will return access request successfully", func(t *testing.T) {
		// Given
		f := setup(t)
		svc := backend.NewDefaultService(f.persister, f.logger)
		arName := "some-accessrequest"
		namespace := "some-namespace"
		ar := newAccessRequest(arName, namespace, "some-app", "some-role", "some-user")
		f.persister.EXPECT().GetAccessRequest(mock.Anything, arName, namespace).Return(ar, nil)

		// When
		result, err := svc.GetAccessRequest(context.Background(), arName, namespace)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, arName, result.GetName())
		assert.Equal(t, namespace, result.GetNamespace())
	})
	t.Run("will return error if k8s request fails", func(t *testing.T) {
		// Given
		f := setup(t)
		svc := backend.NewDefaultService(f.persister, f.logger)
		arName := "some-accessrequest"
		namespace := "some-namespace"
		f.persister.EXPECT().GetAccessRequest(mock.Anything, arName, namespace).
			Return(nil, fmt.Errorf("some internal error"))
		f.logger.EXPECT().Error(mock.Anything, mock.Anything)

		// When
		result, err := svc.GetAccessRequest(context.Background(), arName, namespace)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "some internal error")
	})
	t.Run("will return nil and no error if accessrequest not found", func(t *testing.T) {
		// Given
		f := setup(t)
		svc := backend.NewDefaultService(f.persister, f.logger)
		arName := "some-accessrequest"
		namespace := "some-namespace"

		r := backend.GetAccessRequestResource()

		gr := schema.GroupResource{
			Group:    r.Group,
			Resource: r.Resource,
		}
		notFoundError := errors.NewNotFound(gr, "some-err")
		f.persister.EXPECT().GetAccessRequest(mock.Anything, arName, namespace).
			Return(nil, notFoundError)

		// When
		ar, err := svc.GetAccessRequest(context.Background(), arName, namespace)

		// Then
		assert.NoError(t, err)
		assert.Nil(t, ar)
	})

}
