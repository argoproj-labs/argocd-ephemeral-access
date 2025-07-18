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
	newApp := func(prj string) *argocd.Application {
		return &argocd.Application{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec: argocd.ApplicationSpec{
				Project: prj,
			},
		}
	}
	newRoleTemplate := func(spec api.RoleTemplateSpec) *api.RoleTemplate {
		return &api.RoleTemplate{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       spec,
		}
	}
	newProject := func(roles []argocd.ProjectRole) *argocd.AppProject {
		return &argocd.AppProject{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec: argocd.AppProjectSpec{
				Roles: roles,
			},
		}
	}
	setup := func(clientMock *mocks.MockK8sClient, app *argocd.Application, rt *api.RoleTemplate, prj *argocd.AppProject, updatedProj *argocd.AppProject, updatedAR *api.AccessRequest) {
		clientMock.EXPECT().
			Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Application")).
			RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if app == nil {
					return apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io/v1alpha1", Resource: "Application"}, key.Name)
				}
				appLocal := obj.(*argocd.Application)
				appLocal.Spec = app.Spec
				return nil
			}).Maybe()
		clientMock.EXPECT().
			Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.RoleTemplate")).
			RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				rtLocal := obj.(*api.RoleTemplate)
				rtLocal.Spec = rt.DeepCopy().Spec
				return nil
			}).Maybe()
		clientMock.EXPECT().
			Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject")).
			RunAndReturn(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
				prjLocal := obj.(*argocd.AppProject)
				prjLocal.Spec = prj.DeepCopy().Spec
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

	t.Run("will validate the project", func(t *testing.T) {
		t.Run("will invalidate the AccessRequest if the application project is not set", func(t *testing.T) {
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
		t.Run("will invalidate the AccessRequest and revoke access when application.spec.project changes", func(t *testing.T) {
			// Given
			updatedAR := &api.AccessRequest{}
			updatedProject := &argocd.AppProject{}

			app := newApp("NEW_PROJECT")
			rt := newRoleTemplate(api.RoleTemplateSpec{
				Name:        "some-role-template",
				Description: "some role description",
				Policies:    []string{"policy1", "policy2"},
			})
			prj := newProject(
				[]argocd.ProjectRole{
					{
						Name:        "ephemeral-some-role-template-someAppNs-someApp",
						Description: "some role description",
						Policies:    []string{"policy1", "policy2"},
						JWTTokens:   []argocd.JWTToken{},
						Groups:      []string{"some-user", "user-to-be-removed"},
					},
				},
			)

			clientMock := mocks.NewMockK8sClient(t)
			setup(clientMock, app, rt, prj, updatedProject, updatedAR)
			ar := utils.NewAccessRequest("test", "default", "someApp", "someAppNs", "someRole", "someRoleNs", "user-to-be-removed")
			ar.Status.TargetProject = "someProject"
			ar.Status.RequestState = api.GrantedStatus
			svc := controller.NewService(clientMock, nil, nil)

			// When
			status, err := svc.HandlePermission(context.Background(), ar)

			// Then
			assert.NoError(t, err)
			assert.NotNil(t, status, "status is nil")
			assert.Equal(t, api.InvalidStatus, status, "status must be invalid")
			assert.Equal(t, api.InvalidStatus, updatedAR.Status.RequestState)
			expectedGroups := []string{"some-user"}
			assert.Equal(t, expectedGroups, updatedProject.Spec.Roles[0].Groups, "groups must be updated")
		})
		t.Run("will not remove access if AccessRequest is not granted yet", func(t *testing.T) {
			// Given
			updatedAR := &api.AccessRequest{}
			updatedProject := &argocd.AppProject{}

			app := newApp("NEW_PROJECT")
			rt := newRoleTemplate(api.RoleTemplateSpec{
				Name:        "some-role-template",
				Description: "some role description",
				Policies:    []string{"policy1", "policy2"},
			})
			prj := newProject(
				[]argocd.ProjectRole{
					{
						Name:        "ephemeral-some-role-template-someAppNs-someApp",
						Description: "some role description",
						Policies:    []string{"policy1", "policy2"},
						JWTTokens:   []argocd.JWTToken{},
						Groups:      []string{"some-user", "another-user"},
					},
				},
			)

			clientMock := mocks.NewMockK8sClient(t)
			setup(clientMock, app, rt, prj, updatedProject, updatedAR)
			ar := utils.NewAccessRequest("test", "default", "someApp", "someAppNs", "someRole", "someRoleNs", "new-user")
			ar.Status.TargetProject = "someProject"
			ar.Status.RequestState = api.RequestedStatus
			svc := controller.NewService(clientMock, nil, nil)

			// When
			status, err := svc.HandlePermission(context.Background(), ar)

			// Then
			assert.NoError(t, err)
			assert.NotNil(t, status, "status is nil")
			assert.Equal(t, api.InvalidStatus, status, "status must be invalid")
			assert.Equal(t, api.InvalidStatus, updatedAR.Status.RequestState)
			assert.Len(t, updatedProject.Spec.Roles, 0, "project roles must not be changed")
		})
		t.Run("will invalidate the AccessRequest if project not found", func(t *testing.T) {
			// Given
			updatedAR := &api.AccessRequest{}
			gr := schema.GroupResource{
				Group:    "argoproj.io/v1alpha1",
				Resource: "AppProject",
			}
			notFoundError := apierrors.NewNotFound(gr, "Project not found")

			clientMock := mocks.NewMockK8sClient(t)
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject")).
				Return(notFoundError)
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Application")).
				RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					app := obj.(*argocd.Application)
					app.Spec.Project = "someProject"
					return nil
				})
			resourceWriterMock := mocks.NewMockSubResourceWriter(t)
			resourceWriterMock.EXPECT().Update(mock.Anything, mock.AnythingOfType("*v1alpha1.AccessRequest")).
				RunAndReturn(func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
					ar := obj.(*api.AccessRequest)
					updatedAR.Spec = ar.Spec
					updatedAR.Status = ar.Status
					return nil
				}).Maybe()
			clientMock.EXPECT().Status().Return(resourceWriterMock).Maybe()
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
			assert.NoError(t, err)
			assert.NotNil(t, status, "status is nil")
			assert.Equal(t, api.InvalidStatus, status)
			assert.Equal(t, api.InvalidStatus, updatedAR.Status.RequestState)
		})
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
					app.Spec.Project = "someProject"
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
		t.Run("will add subject back to role if access is granted and revert tampered project changes", func(t *testing.T) {
			// Given
			updatedAR := &api.AccessRequest{}
			updatedProject := &argocd.AppProject{}

			app := newApp("some-project")
			rt := newRoleTemplate(api.RoleTemplateSpec{
				Name:        "some-role-template",
				Description: "some role description",
				Policies:    []string{"policy1", "policy2"},
			})
			prj := newProject(
				[]argocd.ProjectRole{
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
			)

			clientMock := mocks.NewMockK8sClient(t)
			setup(clientMock, app, rt, prj, updatedProject, updatedAR)
			ar := utils.NewAccessRequest("test", "default", "someApp", "someAppNs", "someRole", "someRoleNs", "removed-user")
			ar.Status.TargetProject = "some-project"
			ar.Status.RequestState = api.GrantedStatus
			svc := controller.NewService(clientMock, nil, nil)

			// When
			status, err := svc.HandlePermission(context.Background(), ar)

			// Then
			assert.NoError(t, err)
			assert.NotNil(t, status, "status is nil")
			assert.Equal(t, api.GrantedStatus, status, "status must be granted")
			assert.Len(t, updatedProject.Spec.Roles, 1, "project roles must not be changed")
			assert.Len(t, updatedProject.Spec.Roles[0].Groups, 2, "project role groups must contain the removed user")
			assert.Contains(t, updatedProject.Spec.Roles[0].Groups, "removed-user", "project role groups must contain the removed user")
			assert.Len(t, updatedProject.Spec.Roles[0].Policies, 2, "project role policies must be reverted to the original policies")
			assert.Contains(t, updatedProject.Spec.Roles[0].Policies, "policy1", "project role policies must contain policy1")
			assert.Contains(t, updatedProject.Spec.Roles[0].Policies, "policy2", "project role policies must contain policy2")
			assert.Len(t, updatedProject.Spec.Roles[0].JWTTokens, 0, "project role JWTTokens must be empty")
		})
	})

	t.Run("will handle application not found", func(t *testing.T) {
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
			setup(clientMock, nil, roleTemplate, currentProj, updatedProject, updatedAR)

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
			setup(clientMock, nil, roleTemplate, tamperedProj, updatedProject, updatedAR)

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
			setup(clientMock, nil, roleTemplate, currentProj, updatedProject, updatedAR)

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
		t.Run("will return error if fails to remove argocd access", func(t *testing.T) {
			// Given
			expectedError := errors.New("error updating AppProject")
			clientMock := mocks.NewMockK8sClient(t)
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.AppProject")).
				RunAndReturn(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
					obj = &argocd.AppProject{}
					return nil
				})
			clientMock.EXPECT().
				Get(mock.Anything, mock.Anything, mock.AnythingOfType("*v1alpha1.Application")).
				RunAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					app := obj.(*argocd.Application)
					app.Spec.Project = "someProject"
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
				})
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
