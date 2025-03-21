package controller_test

import (
	"context"
	"errors"
	"testing"
	"time"

	argocd "github.com/argoproj-labs/argocd-ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/argocd-ephemeral-access/internal/controller"
	"github.com/argoproj-labs/argocd-ephemeral-access/test/mocks"
	"github.com/argoproj-labs/argocd-ephemeral-access/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestHandlePermission(t *testing.T) {
	t.Run("will handle access expired", func(t *testing.T) {
		t.Run("will return error if fails to retrieve AppProject", func(t *testing.T) {
			// Given
			expectedError := errors.New("error retrieving AppProject")
			clientMock := mocks.NewMockK8sClient(t)
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject")).
				Return(expectedError)
			svc := controller.NewService(clientMock, nil, nil)
			ar := utils.NewAccessRequest("test", "default", "someApp", "someAppNs", "someRole", "someRoleNs", "")
			past := &metav1.Time{
				Time: time.Now().Add(time.Minute * -1),
			}
			ar.Status.ExpiresAt = past
			ar.Status.TargetProject = "someProject"
			app := &argocd.Application{}

			// When
			status, err := svc.HandlePermission(context.Background(), ar, app, nil)

			// Then
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), expectedError.Error())
			assert.NotNil(t, status, "status is nil")
			assert.Equal(t, "", string(status))
		})
		t.Run("will return error if fails to remove argocd access", func(t *testing.T) {
			// Given
			expectedError := errors.New("error updating AppProject")
			clientMock := mocks.NewMockK8sClient(t)
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject")).
				RunAndReturn(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
					obj = &argocd.AppProject{}
					return nil
				}).
				Once()
			clientMock.EXPECT().
				Patch(mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject"), mock.Anything, mock.Anything).
				Return(expectedError).
				Once()
			svc := controller.NewService(clientMock, nil, nil)
			ar := utils.NewAccessRequest("test", "default", "someApp", "someAppNs", "someRole", "someRoleNs", "")
			past := &metav1.Time{
				Time: time.Now().Add(time.Minute * -1),
			}
			ar.Status.ExpiresAt = past
			ar.Status.TargetProject = "someProject"
			app := &argocd.Application{}
			rt := &api.RoleTemplate{
				Spec: api.RoleTemplateSpec{
					Name:        "some-role",
					Description: "some-role-description",
					Policies:    []string{"some-policy"},
				},
			}

			// When
			status, err := svc.HandlePermission(context.Background(), ar, app, rt)

			// Then
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), expectedError.Error())
			assert.NotNil(t, status, "status is nil")
			assert.Equal(t, "", string(status))
		})
	})
}
