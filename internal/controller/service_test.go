package controller_test

import (
	"context"
	"errors"
	"testing"
	"time"

	argocd "github.com/argoproj-labs/argocd-ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/argocd-ephemeral-access/internal/controller"
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/plugin"
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
			// app := &argocd.Application{}

			// When
			status, err := svc.HandlePermission(context.Background(), ar)

			// Then
			assert.Error(t, err)
			assert.Contains(t, err.Error(), expectedError.Error())
			assert.NotNil(t, status, "status is nil")
			assert.Empty(t, string(status))
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
			// app := &argocd.Application{}
			// rt := &api.RoleTemplate{
			// 	Spec: api.RoleTemplateSpec{
			// 		Name:        "some-role",
			// 		Description: "some-role-description",
			// 		Policies:    []string{"some-policy"},
			// 	},
			// }

			// When
			status, err := svc.HandlePermission(context.Background(), ar)

			// Then
			assert.Error(t, err)
			assert.Contains(t, err.Error(), expectedError.Error())
			assert.NotNil(t, status, "status is nil")
			assert.Empty(t, string(status))
		})
	})
	t.Run("will handle plugins", func(t *testing.T) {
		t.Run("will update the history with the latest plugin message", func(t *testing.T) {
			// Given
			resourceWriterMock := mocks.NewMockSubResourceWriter(t)
			resourceWriterMock.EXPECT().Update(mock.Anything, mock.AnythingOfType("*v1alpha1.AccessRequest")).Return(nil)
			k8sMock := mocks.NewMockK8sClient(t)
			k8sMock.EXPECT().Status().Return(resourceWriterMock)
			pluginMock := mocks.NewMockAccessRequester(t)
			response1 := &plugin.GrantResponse{
				Status:  plugin.GrantStatusPending,
				Message: "message 1",
			}
			pluginMock.EXPECT().
				GrantAccess(mock.AnythingOfType("*v1alpha1.AccessRequest"), mock.AnythingOfType("*v1alpha1.Application")).
				Return(response1, nil).Once()
			pendingMessage := "expected message"

			response2 := &plugin.GrantResponse{
				Status:  plugin.GrantStatusPending,
				Message: pendingMessage,
			}
			pluginMock.EXPECT().
				GrantAccess(mock.AnythingOfType("*v1alpha1.AccessRequest"), mock.AnythingOfType("*v1alpha1.Application")).
				Return(response2, nil).Times(2)

			approvedMessage := "approved"
			response3 := &plugin.GrantResponse{
				Status:  plugin.GrantStatusGranted,
				Message: "approved",
			}
			pluginMock.EXPECT().
				GrantAccess(mock.AnythingOfType("*v1alpha1.AccessRequest"), mock.AnythingOfType("*v1alpha1.Application")).
				Return(response3, nil)

			k8sMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject")).
				RunAndReturn(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
					obj = &argocd.AppProject{}
					return nil
				}).
				Once()
			k8sMock.EXPECT().
				Patch(mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject"), mock.Anything, mock.Anything).
				Return(nil).
				Once()

			svc := controller.NewService(k8sMock, nil, pluginMock)
			ar := utils.NewAccessRequest("test", "default", "someApp", "someAppNs", "someRole", "someRoleNs", "")
			// app := &argocd.Application{}
			// rt := &api.RoleTemplate{
			// 	Spec: api.RoleTemplateSpec{
			// 		Name:        "some-role",
			// 		Description: "some-role-description",
			// 		Policies:    []string{"some-policy"},
			// 	},
			// }

			// When
			svc.HandlePermission(context.Background(), ar)
			svc.HandlePermission(context.Background(), ar)
			svc.HandlePermission(context.Background(), ar)
			status, err := svc.HandlePermission(context.Background(), ar)

			// Then
			assert.NoError(t, err)
			assert.NotNil(t, status, "status is nil")
			assert.Equal(t, api.GrantedStatus, status)
			assert.Equal(t, pendingMessage, ar.GetLastStatusDetails(api.RequestedStatus))
			assert.Equal(t, approvedMessage, ar.GetLastStatusDetails(api.GrantedStatus))
			assert.Len(t, ar.Status.History, 4)
		})
	})
}
