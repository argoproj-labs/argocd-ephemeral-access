package backend_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	argocd "github.com/argoproj-labs/argocd-ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/argocd-ephemeral-access/internal/backend"
	"github.com/argoproj-labs/argocd-ephemeral-access/test/mocks"
	"github.com/argoproj-labs/argocd-ephemeral-access/test/testdata"
	"github.com/argoproj-labs/argocd-ephemeral-access/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
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
	logger.EXPECT().Debug(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	logger.EXPECT().Debug(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
	logger.EXPECT().Info(mock.Anything, mock.Anything).Maybe()
	svc := backend.NewDefaultService(persister, logger, ControllerNamespace, AccessRequestDuration)
	return &serviceFixture{
		persister: persister,
		logger:    logger,
		svc:       svc,
	}
}

func newUnstructured(t *testing.T, fromYaml string) *unstructured.Unstructured {
	t.Helper()
	un := &unstructured.Unstructured{}
	err := yaml.Unmarshal([]byte(fromYaml), un)
	require.NoError(t, err)
	return un
}
func newApp(t *testing.T, name, namespace, labels string) *unstructured.Unstructured {
	t.Helper()
	appYaml := fmt.Sprintf(testdata.AppYAMLTmpl, name, namespace, labels)
	return newUnstructured(t, appYaml)
}
func newAppProject(t *testing.T, name, namespace, labels string) *unstructured.Unstructured {
	t.Helper()
	appProjectYaml := fmt.Sprintf(testdata.AppYAMLTmpl, name, namespace, labels)
	return newUnstructured(t, appProjectYaml)
}

func TestServiceCreateAccessRequest(t *testing.T) {
	t.Run("will create access request successfully", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			UserId:               "some-user-id",
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
		assert.Equal(t, fmt.Sprintf("%s-%s-", key.Username, ab.Spec.RoleTemplateRef.Name), result.GetGenerateName())
		assert.Equal(t, key.Namespace, result.GetNamespace())
		assert.Equal(t, key.ApplicationName, result.Spec.Application.Name)
		assert.Equal(t, key.ApplicationNamespace, result.Spec.Application.Namespace)
		assert.Equal(t, key.UserId, *result.Spec.Subject.UserId)
		assert.Equal(t, key.Username, result.Spec.Subject.Username)
		assert.Equal(t, ab.Spec.FriendlyName, result.Spec.Role.FriendlyName)
		assert.Equal(t, ab.Spec.Ordinal, result.Spec.Role.Ordinal)
		assert.Equal(t, ab.Spec.RoleTemplateRef.Name, result.Spec.Role.TemplateRef.Name)
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
	t.Run("will sort access request", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		ar1 := newAccessRequest(key, "some-role")
		ar1.Status.RequestState = api.GrantedStatus

		ar2 := newAccessRequest(key, "some-role")
		ar2.Status.RequestState = api.RequestedStatus
		ar2.Spec.Role.Ordinal = 2

		ar3 := newAccessRequest(key, "some-role")
		ar3.Status.RequestState = api.InitiatedStatus
		ar3.Spec.Role.Ordinal = 1

		ar4 := newAccessRequest(key, "some-role")
		ar4.Status.RequestState = api.TimeoutStatus

		ar5 := newAccessRequest(key, "some-role")
		ar5.Status.RequestState = api.ExpiredStatus

		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).Return(&api.AccessRequestList{Items: []api.AccessRequest{*ar5, *ar4, *ar3, *ar2, *ar1}}, nil)

		// When
		result, err := f.svc.ListAccessRequests(context.Background(), key, false)
		resultSorted, errSorted := f.svc.ListAccessRequests(context.Background(), key, true)

		// Then
		assert.NoError(t, err)
		assert.NoError(t, errSorted)
		assert.NotNil(t, result)
		assert.NotNil(t, resultSorted)
		assert.Equal(t, 5, len(result))
		assert.Equal(t, 5, len(resultSorted))
		assert.Equal(t, ar5, result[0])
		assert.Equal(t, ar4, result[1])
		assert.Equal(t, ar3.Status.RequestState, resultSorted[0].Status.RequestState)
		assert.Equal(t, ar2.Status.RequestState, resultSorted[1].Status.RequestState)
		assert.Equal(t, ar1.Status.RequestState, resultSorted[2].Status.RequestState)
		assert.Equal(t, ar4.Status.RequestState, resultSorted[3].Status.RequestState)
		assert.Equal(t, ar5.Status.RequestState, resultSorted[4].Status.RequestState)
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
		timeoutAR := newAccessRequest(key, roleName)
		timeoutAR.Status.RequestState = api.TimeoutStatus
		deniedAR := newAccessRequest(key, roleName)
		deniedAR.Status.RequestState = api.DeniedStatus
		expiredAR := newAccessRequest(key, roleName)
		expiredAR.Status.RequestState = api.ExpiredStatus
		invalidAR := newAccessRequest(key, roleName)
		invalidAR.Status.RequestState = api.InvalidStatus
		ar := newAccessRequest(key, roleName)
		f.persister.EXPECT().ListAccessRequests(mock.Anything, key).Return(&api.AccessRequestList{Items: []api.AccessRequest{*deniedAR, *expiredAR, *invalidAR, *timeoutAR, *ar}}, nil)

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

func strPtr(str string) *string {
	return &str
}

func TestServiceGetAccessBindingsForGroups(t *testing.T) {
	ifCondition := "project.metadata.labels != nil && project.metadata.labels['some-company.com/project-id'] != nil && project.metadata.labels['some-company.com/project-id'] != ''"
	accessBindingsInNamespace := &api.AccessBindingList{
		Items: []api.AccessBinding{
			{
				Spec: api.AccessBindingSpec{
					RoleTemplateRef: api.RoleTemplateReference{
						Name: "developer",
					},
					Subjects: []string{
						"{{ index .project.metadata.labels \"some-company.com/project-id\" }}-developer",
					},
					If:           strPtr(ifCondition),
					Ordinal:      10,
					FriendlyName: strPtr("Write (Developer)"),
				},
			},
			{
				Spec: api.AccessBindingSpec{
					RoleTemplateRef: api.RoleTemplateReference{
						Name: "devops",
					},
					Subjects: []string{
						"{{ index .project.metadata.labels \"some-company.com/project-id\" }}-devops",
					},
					If:           strPtr(ifCondition),
					Ordinal:      5,
					FriendlyName: strPtr("Write (DevOps)"),
				},
			},
			{
				Spec: api.AccessBindingSpec{
					RoleTemplateRef: api.RoleTemplateReference{
						Name: "admin",
					},
					Subjects: []string{
						"{{ index .project.metadata.labels \"some-company.com/project-id\" }}-admin",
					},
					If:           strPtr(ifCondition),
					Ordinal:      0,
					FriendlyName: strPtr("Write (Admin)"),
				},
			},
		},
	}
	accessBindingsInControllerNs := &api.AccessBindingList{
		Items: []api.AccessBinding{
			{
				Spec: api.AccessBindingSpec{
					RoleTemplateRef: api.RoleTemplateReference{
						Name: "admin-controller",
					},
					Subjects: []string{
						"{{ index .project.metadata.labels \"some-company.com/project-id\" }}-admin",
					},
					If:           strPtr(ifCondition),
					Ordinal:      0,
					FriendlyName: strPtr("Write (Admin) from controller namespace"),
				},
			},
		},
	}
	t.Run("will return allowed AccessBindings in descending order with multiple matching groups", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		ns := "some-namespace"
		groups := []string{"my-project-developer", "my-project-devops", "my-project-admin"}
		f.persister.EXPECT().ListAllAccessBindings(mock.Anything, ns).
			Return(accessBindingsInNamespace, nil)
		f.persister.EXPECT().ListAllAccessBindings(mock.Anything, ControllerNamespace).
			Return(accessBindingsInControllerNs, nil)
		app := newApp(t, "some-app", "some-ns", "\"some-company.com/project-id\": my-project")
		appproject := newAppProject(t, "some-project", "some-ns", "\"some-company.com/project-id\": my-project")

		// When
		abs, err := f.svc.GetAccessBindingsForGroups(context.Background(), ns, groups, app, appproject)

		require.NoError(t, err)
		require.NotNil(t, abs)
		require.Len(t, abs, 4)
		assert.Equal(t, "Write (Developer)", *abs[0].Spec.FriendlyName)
		assert.Equal(t, "Write (DevOps)", *abs[1].Spec.FriendlyName)
		assert.Equal(t, "Write (Admin)", *abs[2].Spec.FriendlyName)
		assert.Equal(t, "Write (Admin) from controller namespace", *abs[3].Spec.FriendlyName)
	})
	t.Run("will return allowed AccessBindings with one matching group", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		ns := "some-namespace"
		groups := []string{"NOT-MATCHING-developer", "my-project-devops", "NOT-MATCHING-admin"}
		f.persister.EXPECT().ListAllAccessBindings(mock.Anything, ns).
			Return(accessBindingsInNamespace, nil)
		f.persister.EXPECT().ListAllAccessBindings(mock.Anything, ControllerNamespace).
			Return(accessBindingsInControllerNs, nil)
		app := newApp(t, "some-app", "some-ns", "\"some-company.com/project-id\": my-project")
		appproject := newAppProject(t, "some-project", "some-ns", "\"some-company.com/project-id\": my-project")

		// When
		abs, err := f.svc.GetAccessBindingsForGroups(context.Background(), ns, groups, app, appproject)

		require.NoError(t, err)
		require.NotNil(t, abs)
		require.Len(t, abs, 1)
		assert.Equal(t, "Write (DevOps)", *abs[0].Spec.FriendlyName)
	})
	t.Run("will return empty AccessBindings when no matching group", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		ns := "some-namespace"
		groups := []string{"NOT-MATCHING-developer", "NOT-MATCHING-admin"}
		f.persister.EXPECT().ListAllAccessBindings(mock.Anything, ns).
			Return(accessBindingsInNamespace, nil)
		f.persister.EXPECT().ListAllAccessBindings(mock.Anything, ControllerNamespace).
			Return(accessBindingsInControllerNs, nil)
		app := newApp(t, "some-app", "some-ns", "\"some-company.com/project-id\": my-project")
		appproject := newAppProject(t, "some-project", "some-ns", "\"some-company.com/project-id\": my-project")

		// When
		abs, err := f.svc.GetAccessBindingsForGroups(context.Background(), ns, groups, app, appproject)

		require.NoError(t, err)
		require.NotNil(t, abs)
		require.Len(t, abs, 0)
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
		app, err := utils.ToUnstructured(&argocd.Application{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"hello": "world",
				},
			},
		})
		require.NoError(t, err)
		project, err := utils.ToUnstructured(&argocd.AppProject{
			ObjectMeta: metav1.ObjectMeta{
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
		f.logger.EXPECT().Error(mock.Anything, mock.Anything).Run(func(err error, msg string, keysAndValues ...any) {
			errorMsg = msg
		}).Once()

		// When
		result, err := f.svc.GetGrantingAccessBinding(context.Background(), roleName, namespace, groups, app, project)

		// Then
		assert.NoError(t, err)
		assert.Nil(t, result)
		assert.Contains(t, errorMsg, "Cannot render subjects")
	})
}

func TestServiceGetApplication(t *testing.T) {
	t.Run("will return the application when found", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		app := &unstructured.Unstructured{
			Object: map[string]any{
				"hello": "world",
			},
		}
		name := "my-name"
		namespace := "my-namespace"
		f.persister.EXPECT().GetApplication(mock.Anything, name, namespace).Return(app, nil)

		// When
		result, err := f.svc.GetApplication(context.Background(), name, namespace)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, app, result)
	})
	t.Run("will return nil if not found", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		name := "my-name"
		namespace := "my-namespace"
		f.persister.EXPECT().GetApplication(mock.Anything, name, namespace).Return(nil, errors.NewNotFound(schema.GroupResource{}, "some-err"))

		// When
		ar, err := f.svc.GetApplication(context.Background(), name, namespace)

		// Then
		assert.NoError(t, err)
		assert.Nil(t, ar)
	})
	t.Run("will return error if k8s request fails", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		name := "my-name"
		namespace := "my-namespace"
		f.persister.EXPECT().GetApplication(mock.Anything, name, namespace).Return(nil, fmt.Errorf("some internal error"))

		// When
		result, err := f.svc.GetApplication(context.Background(), name, namespace)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "some internal error")
	})
}

func TestServiceGetAppProject(t *testing.T) {
	t.Run("will return the project when found", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		project := &unstructured.Unstructured{
			Object: map[string]any{
				"hello": "world",
			},
		}
		name := "my-name"
		namespace := "my-namespace"
		f.persister.EXPECT().GetAppProject(mock.Anything, name, namespace).Return(project, nil)

		// When
		result, err := f.svc.GetAppProject(context.Background(), name, namespace)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, project, result)
	})
	t.Run("will return nil if not found", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		name := "my-name"
		namespace := "my-namespace"
		f.persister.EXPECT().GetAppProject(mock.Anything, name, namespace).Return(nil, errors.NewNotFound(schema.GroupResource{}, "some-err"))

		// When
		ar, err := f.svc.GetAppProject(context.Background(), name, namespace)

		// Then
		assert.NoError(t, err)
		assert.Nil(t, ar)
	})
	t.Run("will return error if k8s request fails", func(t *testing.T) {
		// Given
		f := serviceSetup(t)
		name := "my-name"
		namespace := "my-namespace"
		f.persister.EXPECT().GetAppProject(mock.Anything, name, namespace).Return(nil, fmt.Errorf("some internal error"))

		// When
		result, err := f.svc.GetAppProject(context.Background(), name, namespace)

		// Then
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "some internal error")
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
		first.Spec.Role.TemplateRef.Name = "a"
		second := base.DeepCopy()
		second.Spec.Role.TemplateRef.Name = "b"
		third := base.DeepCopy()
		third.Spec.Role.TemplateRef.Name = "c"

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
	t.Run("array should be descending ordered by creation for the same status, ordinal and role name", func(t *testing.T) {
		// Given
		base := utils.NewAccessRequestCreated(utils.WithRole())
		first := base.DeepCopy()
		first.CreationTimestamp = metav1.NewTime(metav1.Now().Add(time.Second * 1))
		second := base.DeepCopy()
		second.CreationTimestamp = metav1.NewTime(metav1.Now().Add(time.Second * 2))
		third := base.DeepCopy()
		third.CreationTimestamp = metav1.NewTime(metav1.Now().Add(time.Second * 3))

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
		require.Equal(t, third, items[0])
		require.Equal(t, second, items[1])
		require.Equal(t, first, items[2])
	})
}

func Test_getAccessRequestPrefix(t *testing.T) {
	tests := []struct {
		name     string
		username string
		roleName string
		expected string
	}{
		{
			name:     "should use the first part of email",
			username: "test@argoproj.io",
			roleName: "my-role",
			expected: "test-my-role-",
		},
		{
			name:     "should not exceed max length",
			username: "loremipsumdolorsitametconsecteturadipiscingelitsuspendissetempussemperleoeuvestibulumsemtincidunttinciduntvestibulumaccumsanmaurissedrisusdignissimaliq",
			roleName: "uamaeneanabibendumtellusaeneandapibuslacusetinterdumfeugiatsuspendissevehiculaliberodignissimturpistincidunttristiquepraesentmolestietemporduieugravida",
			expected: "loremipsumdolorsitametconsec-uamaeneanabibendumtellusaene-",
		},
		{
			name:     "should use the maximum amount available for username",
			username: "loremipsumdolorsitametconsecteturadipiscingelitsuspendissetem",
			roleName: "uamaenea",
			expected: "loremipsumdolorsitametconsecteturadipiscingelits-uamaenea-",
		},
		{
			name:     "should use the maximum amount available for roleName",
			username: "uamaenea",
			roleName: "loremipsumdolorsitametconsecteturadipiscingelitsuspe",
			expected: "uamaenea-loremipsumdolorsitametconsecteturadipiscingelits-",
		},
		{
			name:     "should not contain any invalid char",
			username: "my.(user)[1234567890] +_)(*&^%$#@!~",
			roleName: "a-role +_)(*&^%$#@!~",
			expected: "my.user1234567890-a-role-",
		},
		{
			name:     "should not start with a dash",
			username: "---username",
			roleName: "---role",
			expected: "username-role-",
		},
		{
			name:     "should not start with a period",
			username: ".username",
			roleName: "...role",
			expected: "username-role-",
		},
		{
			name:     "should be lowercase",
			username: "UserName",
			roleName: "Role",
			expected: "username-role-",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backend.GetAccessRequestPrefix(tt.username, tt.roleName)
			validationErrors := validation.NameIsDNSSubdomain(got, true)

			assert.Equal(t, tt.expected, got)
			assert.Equalf(t, 0, len(validationErrors), fmt.Sprintf("Validation Errors: \n%s", strings.Join(validationErrors, "\n")))
			assert.LessOrEqual(t, len(got), backend.MaxGeneratedNameLength)
		})
	}
}
