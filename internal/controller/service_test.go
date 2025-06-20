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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestHandlePermission(t *testing.T) {
	t.Run("will set AccessRequest to invalid if the application project is not set", func(t *testing.T) {
		// Given
		updatedAR := &api.AccessRequest{}
		invalidApp := &argocd.Application{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec: argocd.ApplicationSpec{
				Project: "",
			},
		}
		clientMock := mocks.NewMockK8sClient(t)
		clientMock.EXPECT().
			Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Application")).
			RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				app := obj.(*argocd.Application)
				app.Spec = invalidApp.Spec
				return nil
			}).Maybe()
		resourceWriterMock := mocks.NewMockSubResourceWriter(t)
		resourceWriterMock.EXPECT().Update(mock.Anything, mock.AnythingOfType("*v1alpha1.AccessRequest")).
			RunAndReturn(func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
				ar := obj.(*api.AccessRequest)
				updatedAR.Spec = ar.Spec
				updatedAR.Status = ar.Status
				return nil
			}).Maybe()
		clientMock.EXPECT().Status().Return(resourceWriterMock).Maybe()
		ar := utils.NewAccessRequest("test", "default", "someApp", "someAppNs", "someRole", "someRoleNs", "user-to-be-removed")
		svc := controller.NewService(clientMock, nil, nil)

		// When
		status, err := svc.HandlePermission(context.Background(), ar)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, status, "status is nil")
		assert.Equal(t, api.InvalidStatus, status, "status must be invalid")
		assert.Equal(t, api.InvalidStatus, updatedAR.Status.RequestState)
	})
	t.Run("will handle application not found", func(t *testing.T) {
		setup := func(clientMock *mocks.MockK8sClient, roleTemplate *api.RoleTemplate, appProj *argocd.AppProject, updatedProj *argocd.AppProject, updatedAR *api.AccessRequest) {
			gr := schema.GroupResource{
				Group:    "argoproj.io/v1alpha1",
				Resource: "Application",
			}
			notFoundErr := apierrors.NewNotFound(gr, "someApp")
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Application")).
				Return(notFoundErr).Maybe()
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.RoleTemplate")).
				RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					rt := obj.(*api.RoleTemplate)
					rt.Spec = roleTemplate.DeepCopy().Spec
					return nil
				}).Maybe()
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject")).
				RunAndReturn(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
					prj := obj.(*argocd.AppProject)
					prj.Spec = appProj.DeepCopy().Spec
					return nil
				}).Maybe()
			clientMock.EXPECT().
				Patch(mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject"), mock.Anything, mock.Anything).
				RunAndReturn(func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
					updatedProj.Spec = obj.(*argocd.AppProject).Spec
					return nil
				}).Maybe()
			resourceWriterMock := mocks.NewMockSubResourceWriter(t)
			resourceWriterMock.EXPECT().Update(mock.Anything, mock.AnythingOfType("*v1alpha1.AccessRequest")).
				RunAndReturn(func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					ar := obj.(*api.AccessRequest)
					updatedAR.Spec = ar.Spec
					updatedAR.Status = ar.Status
					return nil
				}).Maybe()
			clientMock.EXPECT().Status().Return(resourceWriterMock).Maybe()
		}

		t.Run("will remove Argo CD permissions successfully", func(t *testing.T) {
			// Given
			clientMock := mocks.NewMockK8sClient(t)
			roleTemplate := &api.RoleTemplate{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Spec: api.RoleTemplateSpec{
					Name:        "some-role-template",
					Description: "some role description",
					Policies:    []string{"policy1", "policy2"},
				},
			}
			currentProj := &argocd.AppProject{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Spec: argocd.AppProjectSpec{
					Roles: []argocd.ProjectRole{
						{
							Name:        "ephemeral-some-role-template-someAppNs-someApp",
							Description: "some role description",
							Policies:    []string{"policy1", "policy2"},
							JWTTokens:   []argocd.JWTToken{},
							Groups:      []string{"some-user", "user-to-be-removed"},
						},
					},
				},
			}
			updatedProject := &argocd.AppProject{}
			updatedAR := &api.AccessRequest{}
			setup(clientMock, roleTemplate, currentProj, updatedProject, updatedAR)

			svc := controller.NewService(clientMock, nil, nil)
			ar := utils.NewAccessRequest("test", "default", "someApp", "someAppNs", "someRole", "someRoleNs", "user-to-be-removed")
			ar.Status.TargetProject = "someProject"

			// When
			status, err := svc.HandlePermission(context.Background(), ar)

			// Then
			assert.NoError(t, err)
			assert.NotNil(t, status, "status is nil")
			assert.Equal(t, api.InvalidStatus, status, "status must be invalid")
			assert.Equal(t, api.InvalidStatus, updatedAR.Status.RequestState)
			expectedGroups := []string{"some-user"}
			assert.Equal(t, expectedGroups, updatedProject.Spec.Roles[0].Groups)
		})
		t.Run("will revert tampered project configs", func(t *testing.T) {
			// Given
			clientMock := mocks.NewMockK8sClient(t)
			roleTemplate := &api.RoleTemplate{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Spec: api.RoleTemplateSpec{
					Name:        "some-role-template",
					Description: "some role description",
					Policies:    []string{"policy1", "policy2"},
				},
			}
			// the initial project state was tampered with an additional policy and a JWTToken
			tamperedProj := &argocd.AppProject{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Spec: argocd.AppProjectSpec{
					Roles: []argocd.ProjectRole{
						{
							Name:        "ephemeral-some-role-template-someAppNs-someApp",
							Description: "some role description",
							Policies:    []string{"policy1", "policy2", "tampered-policy"},
							JWTTokens: []argocd.JWTToken{
								{
									IssuedAt:  0,
									ExpiresAt: 0,
									ID:        "tampered-token",
								},
							},
							Groups: []string{"some-user"},
						},
					},
				},
			}
			updatedProject := &argocd.AppProject{}
			updatedAR := &api.AccessRequest{}
			setup(clientMock, roleTemplate, tamperedProj, updatedProject, updatedAR)

			svc := controller.NewService(clientMock, nil, nil)
			ar := utils.NewAccessRequest("test", "default", "someApp", "someAppNs", "someRole", "someRoleNs", "another-user")
			ar.Status.TargetProject = "someProject"

			// When
			status, err := svc.HandlePermission(context.Background(), ar)

			// Then
			assert.NoError(t, err)
			assert.NotNil(t, status, "status is nil")
			assert.Equal(t, api.InvalidStatus, status, "status must be invalid")
			assert.Equal(t, api.InvalidStatus, updatedAR.Status.RequestState)
			expectedProj := tamperedProj.DeepCopy()
			// updated project must have the same policies as defined in the roleTemplate, no JWTTokens
			// and removed the invalid access request user
			expectedProj.Spec = argocd.AppProjectSpec{
				Roles: []argocd.ProjectRole{
					{
						Name:        tamperedProj.Spec.Roles[0].Name,
						Description: roleTemplate.Spec.Description,
						Policies:    roleTemplate.Spec.Policies,
						JWTTokens:   []argocd.JWTToken{},
						Groups:      []string{"some-user"},
					},
				},
			}
			assert.Equal(t, expectedProj, updatedProject)
		})
		t.Run("will update to invalid if the AccessRequest TargetProject is not set", func(t *testing.T) {
			// Given
			clientMock := mocks.NewMockK8sClient(t)
			roleTemplate := &api.RoleTemplate{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Spec: api.RoleTemplateSpec{
					Name:        "some-role-template",
					Description: "some role description",
					Policies:    []string{"policy1", "policy2"},
				},
			}
			currentProj := &argocd.AppProject{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Spec: argocd.AppProjectSpec{
					Roles: []argocd.ProjectRole{
						{
							Name:        "ephemeral-some-role-template-someAppNs-someApp",
							Description: "some role description",
							Policies:    []string{"policy1", "policy2"},
							JWTTokens:   []argocd.JWTToken{},
							Groups:      []string{"some-user", "user-to-be-removed"},
						},
					},
				},
			}
			updatedProject := &argocd.AppProject{}
			updatedAR := &api.AccessRequest{}
			setup(clientMock, roleTemplate, currentProj, updatedProject, updatedAR)

			svc := controller.NewService(clientMock, nil, nil)
			ar := utils.NewAccessRequest("test", "default", "someApp", "someAppNs", "someRole", "someRoleNs", "user-to-be-removed")

			// When
			status, err := svc.HandlePermission(context.Background(), ar)

			// Then
			assert.NoError(t, err)
			assert.NotNil(t, status, "status is nil")
			assert.Equal(t, api.InvalidStatus, status, "status must be invalid")
			assert.Equal(t, api.InvalidStatus, updatedAR.Status.RequestState)
		})
	})
	t.Run("will handle access expired", func(t *testing.T) {
		t.Run("will return error if fails to retrieve AppProject", func(t *testing.T) {
			// Given
			expectedError := errors.New("error retrieving AppProject")
			clientMock := mocks.NewMockK8sClient(t)
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject")).
				Return(expectedError)
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Application")).
				RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					app := obj.(*argocd.Application)
					app.Spec.Project = "some-project"
					return nil
				})
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.RoleTemplate")).
				RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					rt := obj.(*api.RoleTemplate)
					rt.Spec.Name = "some-role"
					return nil
				})
			svc := controller.NewService(clientMock, nil, nil)
			ar := utils.NewAccessRequest("test", "default", "someApp", "someAppNs", "someRole", "someRoleNs", "")
			past := &metav1.Time{
				Time: time.Now().Add(time.Minute * -1),
			}
			ar.Status.ExpiresAt = past
			ar.Status.TargetProject = "someProject"

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
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Application")).
				RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					app := obj.(*argocd.Application)
					app.Spec.Project = "some-project"
					return nil
				})
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.RoleTemplate")).
				RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					rt := obj.(*api.RoleTemplate)
					rt.Spec.Name = "some-role"
					rt.Spec.Description = "some-role-description"
					rt.Spec.Policies = []string{"some-policy"}
					return nil
				})
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
			clientMock := mocks.NewMockK8sClient(t)
			clientMock.EXPECT().Status().Return(resourceWriterMock)
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Application")).
				RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					app := obj.(*argocd.Application)
					app.Spec.Project = "some-project"
					return nil
				})
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.RoleTemplate")).
				RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					rt := obj.(*api.RoleTemplate)
					rt.Spec.Name = "some-role"
					rt.Spec.Description = "some-role-description"
					rt.Spec.Policies = []string{"some-policy"}
					return nil
				})

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

			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject")).
				RunAndReturn(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
					obj = &argocd.AppProject{}
					return nil
				}).
				Once()
			clientMock.EXPECT().
				Patch(mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject"), mock.Anything, mock.Anything).
				Return(nil).
				Once()

			svc := controller.NewService(clientMock, nil, pluginMock)
			ar := utils.NewAccessRequest("test", "default", "someApp", "someAppNs", "someRole", "someRoleNs", "")
			ar.Status.TargetProject = "some-project"

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
