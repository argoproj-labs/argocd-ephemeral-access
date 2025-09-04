package backend_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/argocd-ephemeral-access/internal/backend"
	"github.com/argoproj-labs/argocd-ephemeral-access/test/mocks"
	"github.com/argoproj-labs/argocd-ephemeral-access/test/utils"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type apiFixture struct {
	api     humatest.TestAPI
	service *mocks.MockService
	logger  *mocks.MockLogger
}

func apiSetup(t *testing.T) *apiFixture {
	t.Helper()
	_, api := humatest.New(t)
	service := mocks.NewMockService(t)
	logger := mocks.NewMockLogger(t)
	handler := backend.NewAPIHandler(service, logger)
	backend.RegisterRoutes(api, handler)
	return &apiFixture{
		api:     api,
		service: service,
		logger:  logger,
	}
}

func headers(namespace, userId, username, groups, appNs, appName, projName string) []any {
	required := []any{
		fmt.Sprintf("Argocd-Namespace: %s", namespace),
		fmt.Sprintf("Argocd-Username: %s", username),
		fmt.Sprintf("Argocd-User-Groups: %s", groups),
		fmt.Sprintf("Argocd-Application-Name: %s:%s", appNs, appName),
		fmt.Sprintf("Argocd-Project-Name: %s", projName),
	}

	optional := []any{}
	if userId != "" {
		optional = append(optional, fmt.Sprintf("Argocd-User-Id: %s", userId))
	}

	return append(required, optional...)
}

func newArgoCDHeaders(namespace, userId, username, groups, appNs, appName, projName string) *backend.ArgoCDHeaders {
	return &backend.ArgoCDHeaders{
		ArgoCDNamespace:       namespace,
		ArgoCDUserId:          userId,
		ArgoCDUsername:        username,
		ArgoCDUserGroups:      groups,
		ArgoCDApplicationName: fmt.Sprintf("%s:%s", appNs, appName),
		ArgoCDProjectName:     projName,
	}
}

func getHistoryForStatus(history []api.AccessRequestHistory, status api.Status) *api.AccessRequestHistory {
	for i, h := range history {
		if h.RequestState == status {
			return &history[i]
		}
	}
	return nil
}

func TestApiCreateAccessRequest(t *testing.T) {
	t.Run("will create access request successfully", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		projectName := "some-project"
		roleName := "my-custom-role"
		group := "group1"
		ar := utils.NewAccessRequestCreated(utils.WithName("created"))
		arBinding := newDefaultAccessBinding()
		key := &backend.AccessRequestKey{
			Namespace:            ar.GetNamespace(),
			ApplicationName:      ar.Spec.Application.Name,
			ApplicationNamespace: ar.Spec.Application.Namespace,
			UserId:               *ar.Spec.Subject.UserId,
			Username:             ar.Spec.Subject.Username,
		}
		headers := headers(key.Namespace, key.UserId, key.Username, group, key.ApplicationNamespace, key.ApplicationName, projectName)
		project := &unstructured.Unstructured{}
		app := &unstructured.Unstructured{}
		f.service.EXPECT().GetAccessRequestByRole(mock.Anything, key, roleName).Return(nil, nil)
		f.service.EXPECT().GetApplication(mock.Anything, key.ApplicationName, key.ApplicationNamespace).Return(app, nil)
		f.service.EXPECT().GetAppProject(mock.Anything, projectName, key.Namespace).Return(project, nil)
		f.service.EXPECT().GetGrantingAccessBinding(mock.Anything, roleName, key.Namespace, []string{group}, app, project).Return(arBinding, nil)
		f.service.EXPECT().CreateAccessRequest(mock.Anything, key, arBinding).Return(ar, nil)

		// When
		payload := backend.CreateAccessRequestBody{
			RoleName: roleName,
		}
		resp := f.api.Post("/accessrequests", append(headers, payload)...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 200, resp.Result().StatusCode)
		var respBody backend.AccessRequestResponseBody
		err := json.Unmarshal(resp.Body.Bytes(), &respBody)
		assert.NoError(t, err)
		assert.Equal(t, ar.GetNamespace(), respBody.Namespace)
		assert.Equal(t, ar.GetName(), respBody.Name)
	})

	t.Run("will return 422 on invalid headers", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		roleName := "my-custom-role"
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app:with-invalid-colon",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		headers := headers(key.Namespace, key.UserId, key.Username, "group1", key.ApplicationNamespace, key.ApplicationName, "some-project")
		headers = headers[1:]

		// When
		payload := backend.CreateAccessRequestBody{
			RoleName: roleName,
		}
		resp := f.api.Post("/accessrequests", append(headers, payload)...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 422, resp.Result().StatusCode)
	})
	t.Run("will return 400 on invalid header format", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		roleName := "my-custom-role"
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app:with-invalid-colon",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		headers := headers(key.Namespace, key.UserId, key.Username, "group1", key.ApplicationNamespace, key.ApplicationName, "some-project")

		// When
		payload := backend.CreateAccessRequestBody{
			RoleName: roleName,
		}
		resp := f.api.Post("/accessrequests", append(headers, payload)...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 400, resp.Result().StatusCode)
	})
	t.Run("will return 400 on invalid application reference", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		projectName := "some-project"
		roleName := "my-custom-role"
		group := "group1"
		ar := utils.NewAccessRequestCreated(utils.WithName("created"))
		key := &backend.AccessRequestKey{
			Namespace:            ar.GetNamespace(),
			ApplicationName:      ar.Spec.Application.Name,
			ApplicationNamespace: ar.Spec.Application.Namespace,
			Username:             ar.Spec.Subject.Username,
		}
		headers := headers(key.Namespace, key.UserId, key.Username, group, key.ApplicationNamespace, key.ApplicationName, projectName)
		f.service.EXPECT().GetAccessRequestByRole(mock.Anything, key, roleName).Return(nil, nil)
		f.service.EXPECT().GetApplication(mock.Anything, key.ApplicationName, key.ApplicationNamespace).Return(nil, nil)

		// When
		payload := backend.CreateAccessRequestBody{
			RoleName: roleName,
		}
		resp := f.api.Post("/accessrequests", append(headers, payload)...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 400, resp.Result().StatusCode)
	})
	t.Run("will return 400 on invalid project reference", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		projectName := "some-project"
		roleName := "my-custom-role"
		group := "group1"
		ar := utils.NewAccessRequestCreated(utils.WithName("created"))
		key := &backend.AccessRequestKey{
			Namespace:            ar.GetNamespace(),
			ApplicationName:      ar.Spec.Application.Name,
			ApplicationNamespace: ar.Spec.Application.Namespace,
			Username:             ar.Spec.Subject.Username,
		}
		headers := headers(key.Namespace, key.UserId, key.Username, group, key.ApplicationNamespace, key.ApplicationName, projectName)
		app := &unstructured.Unstructured{}
		f.service.EXPECT().GetAccessRequestByRole(mock.Anything, key, roleName).Return(nil, nil)
		f.service.EXPECT().GetApplication(mock.Anything, key.ApplicationName, key.ApplicationNamespace).Return(app, nil)
		f.service.EXPECT().GetAppProject(mock.Anything, projectName, key.Namespace).Return(nil, nil)

		// When
		payload := backend.CreateAccessRequestBody{
			RoleName: roleName,
		}
		resp := f.api.Post("/accessrequests", append(headers, payload)...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 400, resp.Result().StatusCode)
	})
	t.Run("will return 409 if access request already exist for the requested role", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		projectName := "some-project"
		roleName := "my-custom-role"
		group := "group1"
		ar := utils.NewAccessRequestCreated(utils.WithName("created"))
		key := &backend.AccessRequestKey{
			Namespace:            ar.GetNamespace(),
			ApplicationName:      ar.Spec.Application.Name,
			ApplicationNamespace: ar.Spec.Application.Namespace,
			Username:             ar.Spec.Subject.Username,
		}
		headers := headers(key.Namespace, key.UserId, key.Username, group, key.ApplicationNamespace, key.ApplicationName, projectName)
		f.service.EXPECT().GetAccessRequestByRole(mock.Anything, key, roleName).Return(ar, nil)

		// When
		payload := backend.CreateAccessRequestBody{
			RoleName: roleName,
		}
		resp := f.api.Post("/accessrequests", append(headers, payload)...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 409, resp.Result().StatusCode)
	})
	t.Run("will return 403 if access request is not allowed for user", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		projectName := "some-project"
		roleName := "my-custom-role"
		group := "group1"
		ar := utils.NewAccessRequestCreated(utils.WithName("created"))
		key := &backend.AccessRequestKey{
			Namespace:            ar.GetNamespace(),
			ApplicationName:      ar.Spec.Application.Name,
			ApplicationNamespace: ar.Spec.Application.Namespace,
			Username:             ar.Spec.Subject.Username,
		}
		headers := headers(key.Namespace, key.UserId, key.Username, group, key.ApplicationNamespace, key.ApplicationName, projectName)
		project := &unstructured.Unstructured{}
		app := &unstructured.Unstructured{}
		f.service.EXPECT().GetAccessRequestByRole(mock.Anything, key, roleName).Return(nil, nil)
		f.service.EXPECT().GetApplication(mock.Anything, key.ApplicationName, key.ApplicationNamespace).Return(app, nil)
		f.service.EXPECT().GetAppProject(mock.Anything, projectName, key.Namespace).Return(project, nil)
		f.service.EXPECT().GetGrantingAccessBinding(mock.Anything, roleName, key.Namespace, []string{group}, app, project).Return(nil, nil)

		// When
		payload := backend.CreateAccessRequestBody{
			RoleName: roleName,
		}
		resp := f.api.Post("/accessrequests", append(headers, payload)...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 403, resp.Result().StatusCode)
	})
	t.Run("will return 500 on service error getting application", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		projectName := "some-project"
		roleName := "my-custom-role"
		group := "group1"
		ar := utils.NewAccessRequestCreated(utils.WithName("created"))
		key := &backend.AccessRequestKey{
			Namespace:            ar.GetNamespace(),
			ApplicationName:      ar.Spec.Application.Name,
			ApplicationNamespace: ar.Spec.Application.Namespace,
			Username:             ar.Spec.Subject.Username,
		}
		headers := headers(key.Namespace, key.UserId, key.Username, group, key.ApplicationNamespace, key.ApplicationName, projectName)
		f.service.EXPECT().GetAccessRequestByRole(mock.Anything, key, roleName).Return(nil, nil)
		f.service.EXPECT().GetApplication(mock.Anything, key.ApplicationName, key.ApplicationNamespace).Return(nil, fmt.Errorf("some-error"))
		f.logger.EXPECT().Error(mock.Anything, mock.Anything)

		// When
		payload := backend.CreateAccessRequestBody{
			RoleName: roleName,
		}
		resp := f.api.Post("/accessrequests", append(headers, payload)...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 500, resp.Result().StatusCode)
	})
	t.Run("will return 500 on service error getting project", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		projectName := "some-project"
		roleName := "my-custom-role"
		group := "group1"
		ar := utils.NewAccessRequestCreated(utils.WithName("created"))
		key := &backend.AccessRequestKey{
			Namespace:            ar.GetNamespace(),
			ApplicationName:      ar.Spec.Application.Name,
			ApplicationNamespace: ar.Spec.Application.Namespace,
			Username:             ar.Spec.Subject.Username,
		}
		headers := headers(key.Namespace, key.UserId, key.Username, group, key.ApplicationNamespace, key.ApplicationName, projectName)
		app := &unstructured.Unstructured{}
		f.service.EXPECT().GetAccessRequestByRole(mock.Anything, key, roleName).Return(nil, nil)
		f.service.EXPECT().GetApplication(mock.Anything, key.ApplicationName, key.ApplicationNamespace).Return(app, nil)
		f.service.EXPECT().GetAppProject(mock.Anything, projectName, key.Namespace).Return(nil, fmt.Errorf("some-error"))
		f.logger.EXPECT().Error(mock.Anything, mock.Anything)

		// When
		payload := backend.CreateAccessRequestBody{
			RoleName: roleName,
		}
		resp := f.api.Post("/accessrequests", append(headers, payload)...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 500, resp.Result().StatusCode)
	})
	t.Run("will return 500 on service error getting role bindings", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		projectName := "some-project"
		roleName := "my-custom-role"
		group := "group1"
		ar := utils.NewAccessRequestCreated(utils.WithName("created"))
		key := &backend.AccessRequestKey{
			Namespace:            ar.GetNamespace(),
			ApplicationName:      ar.Spec.Application.Name,
			ApplicationNamespace: ar.Spec.Application.Namespace,
			Username:             ar.Spec.Subject.Username,
		}
		headers := headers(key.Namespace, key.UserId, key.Username, group, key.ApplicationNamespace, key.ApplicationName, projectName)
		project := &unstructured.Unstructured{}
		app := &unstructured.Unstructured{}
		f.service.EXPECT().GetAccessRequestByRole(mock.Anything, key, roleName).Return(nil, nil)
		f.service.EXPECT().GetApplication(mock.Anything, key.ApplicationName, key.ApplicationNamespace).Return(app, nil)
		f.service.EXPECT().GetAppProject(mock.Anything, projectName, key.Namespace).Return(project, nil)
		f.service.EXPECT().GetGrantingAccessBinding(mock.Anything, roleName, key.Namespace, []string{group}, app, project).Return(nil, fmt.Errorf("some-error"))
		f.logger.EXPECT().Error(mock.Anything, mock.Anything)

		// When
		payload := backend.CreateAccessRequestBody{
			RoleName: roleName,
		}
		resp := f.api.Post("/accessrequests", append(headers, payload)...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 500, resp.Result().StatusCode)
	})
	t.Run("will return 500 on service error getting access request", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		projectName := "some-project"
		roleName := "my-custom-role"
		group := "group1"
		ar := utils.NewAccessRequestCreated(utils.WithName("created"))
		key := &backend.AccessRequestKey{
			Namespace:            ar.GetNamespace(),
			ApplicationName:      ar.Spec.Application.Name,
			ApplicationNamespace: ar.Spec.Application.Namespace,
			Username:             ar.Spec.Subject.Username,
		}
		headers := headers(key.Namespace, key.UserId, key.Username, group, key.ApplicationNamespace, key.ApplicationName, projectName)
		f.service.EXPECT().GetAccessRequestByRole(mock.Anything, key, roleName).Return(nil, fmt.Errorf("some-error"))
		f.logger.EXPECT().Error(mock.Anything, mock.Anything)

		// When
		payload := backend.CreateAccessRequestBody{
			RoleName: roleName,
		}
		resp := f.api.Post("/accessrequests", append(headers, payload)...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 500, resp.Result().StatusCode)
	})
	t.Run("will return 500 on service error creating access request", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		projectName := "some-project"
		roleName := "my-custom-role"
		group := "group1"
		ar := utils.NewAccessRequestCreated(utils.WithName("created"))
		arBinding := newDefaultAccessBinding()
		key := &backend.AccessRequestKey{
			Namespace:            ar.GetNamespace(),
			ApplicationName:      ar.Spec.Application.Name,
			ApplicationNamespace: ar.Spec.Application.Namespace,
			Username:             ar.Spec.Subject.Username,
		}
		headers := headers(key.Namespace, key.UserId, key.Username, group, key.ApplicationNamespace, key.ApplicationName, projectName)
		project := &unstructured.Unstructured{}
		app := &unstructured.Unstructured{}
		f.service.EXPECT().GetAccessRequestByRole(mock.Anything, key, roleName).Return(nil, nil)
		f.service.EXPECT().GetApplication(mock.Anything, key.ApplicationName, key.ApplicationNamespace).Return(app, nil)
		f.service.EXPECT().GetAppProject(mock.Anything, projectName, key.Namespace).Return(project, nil)
		f.service.EXPECT().GetGrantingAccessBinding(mock.Anything, roleName, key.Namespace, []string{group}, app, project).Return(arBinding, nil)
		f.service.EXPECT().CreateAccessRequest(mock.Anything, key, arBinding).Return(nil, fmt.Errorf("some-error"))
		f.logger.EXPECT().Error(mock.Anything, mock.Anything)

		// When
		payload := backend.CreateAccessRequestBody{
			RoleName: roleName,
		}
		resp := f.api.Post("/accessrequests", append(headers, payload)...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 500, resp.Result().StatusCode)
	})
}

func TestApiListAccessRequest(t *testing.T) {
	t.Run("will return access requests successfully", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		ar1 := utils.NewAccessRequestRequested(utils.WithName("first"))
		ar2 := utils.NewAccessRequestRequested(utils.WithName("second"))
		key := &backend.AccessRequestKey{
			Namespace:            ar1.GetNamespace(),
			ApplicationName:      ar1.Spec.Application.Name,
			ApplicationNamespace: ar1.Spec.Application.Namespace,
			Username:             ar1.Spec.Subject.Username,
		}
		headers := headers(key.Namespace, key.UserId, key.Username, "group1", key.ApplicationNamespace, key.ApplicationName, "some-project")
		f.service.EXPECT().ListAccessRequests(mock.Anything, key, true).Return([]*api.AccessRequest{ar1, ar2}, nil)

		// When
		resp := f.api.Get("/accessrequests", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 200, resp.Result().StatusCode)
		var respBody backend.ListAccessRequestResponseBody
		err := json.Unmarshal(resp.Body.Bytes(), &respBody)
		assert.NoError(t, err)
		require.Equal(t, 2, len(respBody.Items))
		assert.Equal(t, ar1.GetNamespace(), respBody.Items[0].Namespace)
		assert.Equal(t, ar1.GetName(), respBody.Items[0].Name)
		assert.Equal(t, ar2.GetNamespace(), respBody.Items[1].Namespace)
		assert.Equal(t, ar2.GetName(), respBody.Items[1].Name)
	})
	t.Run("will return 422 on invalid headers", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app:with-invalid-colon",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		headers := headers(key.Namespace, key.UserId, key.Username, "group1", key.ApplicationNamespace, key.ApplicationName, "some-project")
		headers = headers[1:]

		// When
		resp := f.api.Get("/accessrequests", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 422, resp.Result().StatusCode)
	})
	t.Run("will return 400 on invalid header format", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app:with-invalid-colon",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		headers := headers(key.Namespace, key.UserId, key.Username, "group1", key.ApplicationNamespace, key.ApplicationName, "some-project")

		// When
		resp := f.api.Get("/accessrequests", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 400, resp.Result().StatusCode)
	})
	t.Run("will return 500 on service error", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		headers := headers(key.Namespace, key.UserId, key.Username, "group1", key.ApplicationNamespace, key.ApplicationName, "some-project")
		f.service.EXPECT().ListAccessRequests(mock.Anything, key, mock.Anything).Return(nil, fmt.Errorf("some-error"))
		f.logger.EXPECT().Error(mock.Anything, mock.Anything)

		// When
		resp := f.api.Get("/accessrequests", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 500, resp.Result().StatusCode)
	})
	t.Run("will return empty list if no access request exist", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		headers := headers(key.Namespace, key.UserId, key.Username, "group1", key.ApplicationNamespace, key.ApplicationName, "some-project")
		f.service.EXPECT().ListAccessRequests(mock.Anything, key, mock.Anything).Return(nil, nil)

		// When
		resp := f.api.Get("/accessrequests", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 200, resp.Result().StatusCode)
		var respBody backend.ListAccessRequestResponseBody
		err := json.Unmarshal(resp.Body.Bytes(), &respBody)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(respBody.Items))
	})

}

func TestArgoCDHeaders_Application(t *testing.T) {
	tests := []struct {
		name              string
		headers           *backend.ArgoCDHeaders
		expectedNamespace string
		expectedName      string
		wantErr           bool
	}{
		{
			name:              "Argocd-Application-Name parsed correctly",
			headers:           newArgoCDHeaders("ns", "userid", "username", "group", "appNs", "appName", "project"),
			expectedNamespace: "appNs",
			expectedName:      "appName",
		},
		{
			name:    "Argocd-Application-Name error when invalid",
			headers: newArgoCDHeaders("ns", "userid", "username", "group", "appNs", "app:Name", "project"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNamespace, gotName, err := tt.headers.Application()
			if (err != nil) != tt.wantErr {
				t.Errorf("ArgoCDHeaders.Application() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotNamespace != tt.expectedNamespace {
				t.Errorf("ArgoCDHeaders.Application() gotNamespace = %v, want %v", gotNamespace, tt.expectedNamespace)
			}
			if gotName != tt.expectedName {
				t.Errorf("ArgoCDHeaders.Application() gotName = %v, want %v", gotName, tt.expectedName)
			}
		})
	}
}

func TestArgoCDHeaders_Groups(t *testing.T) {
	tests := []struct {
		name     string
		headers  *backend.ArgoCDHeaders
		expected []string
	}{
		{
			name:     "Argocd-User-Groups parsed correctly",
			headers:  newArgoCDHeaders("ns", "userid", "username", "group1,group2,group3", "appNs", "appName", "project"),
			expected: []string{"group1", "group2", "group3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.headers.Groups(); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ArgoCDHeaders.Groups() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func Test_toAccessRequestResponseBody(t *testing.T) {
	tests := []struct {
		name          string
		accessRequest *api.AccessRequest
		expected      func(ar *api.AccessRequest) backend.AccessRequestResponseBody
	}{
		{
			name:          "access request after creation",
			accessRequest: utils.NewAccessRequestCreated(),
			expected: func(ar *api.AccessRequest) backend.AccessRequestResponseBody {
				return backend.AccessRequestResponseBody{
					Name:        ar.GetName(),
					Namespace:   ar.GetNamespace(),
					Username:    ar.Spec.Subject.Username,
					Role:        ar.Spec.Role.TemplateRef.Name,
					Permission:  ar.Spec.Role.TemplateRef.Name,
					RequestedAt: "",
					Status:      "",
					ExpiresAt:   "",
					Message:     "",
				}
			},
		},
		{
			name:          "access request after creation with role",
			accessRequest: utils.NewAccessRequestCreated(utils.WithRole()),
			expected: func(ar *api.AccessRequest) backend.AccessRequestResponseBody {
				return backend.AccessRequestResponseBody{
					Name:        ar.GetName(),
					Namespace:   ar.GetNamespace(),
					Username:    ar.Spec.Subject.Username,
					Role:        ar.Spec.Role.TemplateRef.Name,
					Permission:  *ar.Spec.Role.FriendlyName,
					RequestedAt: "",
					Status:      "",
					ExpiresAt:   "",
					Message:     "",
				}
			},
		},
		{
			name:          "access request invalid",
			accessRequest: utils.NewAccessRequestInvalid(utils.WithRole()),
			expected: func(ar *api.AccessRequest) backend.AccessRequestResponseBody {
				return backend.AccessRequestResponseBody{
					Name:        ar.GetName(),
					Namespace:   ar.GetNamespace(),
					Username:    ar.Spec.Subject.Username,
					Role:        ar.Spec.Role.TemplateRef.Name,
					Permission:  *ar.Spec.Role.FriendlyName,
					RequestedAt: "",
					Status:      strings.ToUpper(string(ar.Status.RequestState)),
					ExpiresAt:   "",
					Message:     *getHistoryForStatus(ar.Status.History, api.InvalidStatus).Details,
				}
			},
		},
		{
			name:          "access request initiated",
			accessRequest: utils.NewAccessRequestInitiated(utils.WithRole()),
			expected: func(ar *api.AccessRequest) backend.AccessRequestResponseBody {
				return backend.AccessRequestResponseBody{
					Name:        ar.GetName(),
					Namespace:   ar.GetNamespace(),
					Username:    ar.Spec.Subject.Username,
					Role:        ar.Spec.Role.TemplateRef.Name,
					Permission:  *ar.Spec.Role.FriendlyName,
					RequestedAt: getHistoryForStatus(ar.Status.History, api.InitiatedStatus).TransitionTime.Format(time.RFC3339),
					Status:      strings.ToUpper(string(ar.Status.RequestState)),
					ExpiresAt:   "",
					Message:     "",
				}
			},
		},
		{
			name:          "access request requested",
			accessRequest: utils.NewAccessRequestRequested(utils.WithRole()),
			expected: func(ar *api.AccessRequest) backend.AccessRequestResponseBody {
				return backend.AccessRequestResponseBody{
					Name:        ar.GetName(),
					Namespace:   ar.GetNamespace(),
					Username:    ar.Spec.Subject.Username,
					Role:        ar.Spec.Role.TemplateRef.Name,
					Permission:  *ar.Spec.Role.FriendlyName,
					RequestedAt: getHistoryForStatus(ar.Status.History, api.RequestedStatus).TransitionTime.Format(time.RFC3339),
					Status:      strings.ToUpper(string(ar.Status.RequestState)),
					ExpiresAt:   "",
					Message:     "",
				}
			},
		},
		{
			name:          "access request granted",
			accessRequest: utils.NewAccessRequestGranted(utils.WithRole()),
			expected: func(ar *api.AccessRequest) backend.AccessRequestResponseBody {
				return backend.AccessRequestResponseBody{
					Name:        ar.GetName(),
					Namespace:   ar.GetNamespace(),
					Username:    ar.Spec.Subject.Username,
					Role:        ar.Spec.Role.TemplateRef.Name,
					Permission:  *ar.Spec.Role.FriendlyName,
					RequestedAt: getHistoryForStatus(ar.Status.History, api.InitiatedStatus).TransitionTime.Format(time.RFC3339),
					Status:      strings.ToUpper(string(ar.Status.RequestState)),
					ExpiresAt:   ar.Status.ExpiresAt.Format(time.RFC3339),
					Message:     "",
				}
			},
		},
		{
			name:          "access request expired",
			accessRequest: utils.NewAccessRequestExpired(utils.WithRole()),
			expected: func(ar *api.AccessRequest) backend.AccessRequestResponseBody {
				return backend.AccessRequestResponseBody{
					Name:        ar.GetName(),
					Namespace:   ar.GetNamespace(),
					Username:    ar.Spec.Subject.Username,
					Role:        ar.Spec.Role.TemplateRef.Name,
					Permission:  *ar.Spec.Role.FriendlyName,
					RequestedAt: getHistoryForStatus(ar.Status.History, api.InitiatedStatus).TransitionTime.Format(time.RFC3339),
					Status:      strings.ToUpper(string(ar.Status.RequestState)),
					ExpiresAt:   ar.Status.ExpiresAt.Format(time.RFC3339),
					Message:     "",
				}
			},
		},
		{
			name:          "access request denied",
			accessRequest: utils.NewAccessRequestDenied(utils.WithRole()),
			expected: func(ar *api.AccessRequest) backend.AccessRequestResponseBody {
				return backend.AccessRequestResponseBody{
					Name:        ar.GetName(),
					Namespace:   ar.GetNamespace(),
					Username:    ar.Spec.Subject.Username,
					Role:        ar.Spec.Role.TemplateRef.Name,
					Permission:  *ar.Spec.Role.FriendlyName,
					RequestedAt: getHistoryForStatus(ar.Status.History, api.InitiatedStatus).TransitionTime.Format(time.RFC3339),
					Status:      strings.ToUpper(string(ar.Status.RequestState)),
					ExpiresAt:   "",
					Message:     *getHistoryForStatus(ar.Status.History, api.DeniedStatus).Details,
				}
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			expected := tt.expected(tt.accessRequest)
			if got := backend.ToAccessRequestResponseBody(tt.accessRequest); !reflect.DeepEqual(got, expected) {
				t.Errorf("%s test mismatch:\nwant: %v\ngot:  %v", tt.name, expected, got)
			}
		})
	}
}

func TestApiListAllowedRoles(t *testing.T) {
	newAccessBinding := func(roleName, friendlyName string) *api.AccessBinding {
		return &api.AccessBinding{
			Spec: api.AccessBindingSpec{
				RoleTemplateRef: api.RoleTemplateReference{
					Name: roleName,
				},
				FriendlyName: strPtr(friendlyName),
			},
		}
	}
	t.Run("will return access requests successfully", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		argocdNamespace := "argocd-namespace"
		username := "some-user"
		userId := "some-user-id"
		groups := "group1,group2,group3"
		groupsList := strings.Split(groups, ",")
		appNamespace := "some-namespace"
		appName := "some-app"
		projectName := "some-project"
		role1 := "role-1"
		role2 := "role-2"
		abList := []*api.AccessBinding{
			newAccessBinding(role1, role1),
			newAccessBinding(role2, role2),
		}
		app := &unstructured.Unstructured{}
		appproj := &unstructured.Unstructured{}
		headers := headers(argocdNamespace, userId, username, groups, appNamespace, appName, projectName)
		f.service.EXPECT().GetApplication(mock.Anything, appName, appNamespace).Return(app, nil)
		f.service.EXPECT().GetAppProject(mock.Anything, projectName, argocdNamespace).Return(appproj, nil)
		f.service.EXPECT().GetAccessBindingsForGroups(mock.Anything, argocdNamespace, groupsList, app, appproj).Return(abList, nil)

		// When
		resp := f.api.Get("/roles", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 200, resp.Result().StatusCode)
		var respBody backend.ListAllowedRolesResponseBody
		err := json.Unmarshal(resp.Body.Bytes(), &respBody)
		assert.NoError(t, err)
		require.Equal(t, 2, len(respBody.Items))
		assert.Equal(t, role1, respBody.Items[0].RoleName)
		assert.Equal(t, role2, respBody.Items[1].RoleName)
	})
	t.Run("will return 422 if the groups header is empty", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		argocdNamespace := "argocd-namespace"
		username := "some-user"
		userId := "some-user-id"
		groups := ""
		appNamespace := "some-namespace"
		appName := "some-app"
		projectName := "some-project"
		headers := headers(argocdNamespace, userId, username, groups, appNamespace, appName, projectName)

		// When
		resp := f.api.Get("/roles", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 422, resp.Result().StatusCode)
	})
	t.Run("will return 400 if the application header do not have namespace", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		argocdNamespace := "argocd-namespace"
		username := "some-user"
		userId := "some-user-id"
		groups := "group1,group2"
		appNamespace := ""
		appName := "some-app"
		projectName := "some-project"
		headers := headers(argocdNamespace, userId, username, groups, appNamespace, appName, projectName)

		// When
		resp := f.api.Get("/roles", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 400, resp.Result().StatusCode)
	})
	t.Run("will return 400 if the application header do not have name", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		argocdNamespace := "argocd-namespace"
		username := "some-user"
		userId := "some-user-id"
		groups := "group1,group2"
		appNamespace := "some-namespace"
		appName := ""
		projectName := "some-project"
		headers := headers(argocdNamespace, userId, username, groups, appNamespace, appName, projectName)

		// When
		resp := f.api.Get("/roles", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 400, resp.Result().StatusCode)
	})
	t.Run("will return 500 if it fails to retrieve the application", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		argocdNamespace := "argocd-namespace"
		username := "some-user"
		userId := "some-user-id"
		groups := "group1,group2,group3"
		appNamespace := "some-namespace"
		appName := "some-app"
		projectName := "some-project"
		headers := headers(argocdNamespace, userId, username, groups, appNamespace, appName, projectName)
		f.service.EXPECT().GetApplication(mock.Anything, appName, appNamespace).Return(nil, errors.New("some error"))
		f.logger.EXPECT().Error(mock.Anything, mock.Anything)

		// When
		resp := f.api.Get("/roles", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 500, resp.Result().StatusCode)
	})
	t.Run("will return 404 if application not found", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		argocdNamespace := "argocd-namespace"
		username := "some-user"
		userId := "some-user-id"
		groups := "group1,group2,group3"
		appNamespace := "some-namespace"
		appName := "some-app"
		projectName := "some-project"
		headers := headers(argocdNamespace, userId, username, groups, appNamespace, appName, projectName)
		f.service.EXPECT().GetApplication(mock.Anything, appName, appNamespace).Return(nil, nil)

		// When
		resp := f.api.Get("/roles", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 404, resp.Result().StatusCode)
	})
	t.Run("will return 500 if it fails to retrieve the project", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		argocdNamespace := "argocd-namespace"
		username := "some-user"
		userId := "some-user-id"
		groups := "group1,group2,group3"
		appNamespace := "some-namespace"
		appName := "some-app"
		projectName := "some-project"
		app := &unstructured.Unstructured{}
		headers := headers(argocdNamespace, userId, username, groups, appNamespace, appName, projectName)
		f.service.EXPECT().GetApplication(mock.Anything, appName, appNamespace).Return(app, nil)
		f.service.EXPECT().GetAppProject(mock.Anything, projectName, argocdNamespace).Return(nil, errors.New("some error"))
		f.logger.EXPECT().Error(mock.Anything, mock.Anything)

		// When
		resp := f.api.Get("/roles", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 500, resp.Result().StatusCode)
	})
	t.Run("will return 404 if project not found", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		argocdNamespace := "argocd-namespace"
		username := "some-user"
		userId := "some-user-id"
		groups := "group1,group2,group3"
		appNamespace := "some-namespace"
		appName := "some-app"
		projectName := "some-project"
		app := &unstructured.Unstructured{}
		headers := headers(argocdNamespace, userId, username, groups, appNamespace, appName, projectName)
		f.service.EXPECT().GetApplication(mock.Anything, appName, appNamespace).Return(app, nil)
		f.service.EXPECT().GetAppProject(mock.Anything, projectName, argocdNamespace).Return(nil, nil)

		// When
		resp := f.api.Get("/roles", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 404, resp.Result().StatusCode)
	})
	t.Run("will return 500 if it fails to retrieve the accessbindings list", func(t *testing.T) {
		// Given
		f := apiSetup(t)
		argocdNamespace := "argocd-namespace"
		username := "some-user"
		userId := "some-user-id"
		groups := "group1,group2,group3"
		groupsList := strings.Split(groups, ",")
		appNamespace := "some-namespace"
		appName := "some-app"
		projectName := "some-project"
		app := &unstructured.Unstructured{}
		appproj := &unstructured.Unstructured{}
		headers := headers(argocdNamespace, userId, username, groups, appNamespace, appName, projectName)
		f.service.EXPECT().GetApplication(mock.Anything, appName, appNamespace).Return(app, nil)
		f.service.EXPECT().GetAppProject(mock.Anything, projectName, argocdNamespace).Return(appproj, nil)
		f.service.EXPECT().GetAccessBindingsForGroups(mock.Anything, argocdNamespace, groupsList, app, appproj).Return(nil, errors.New("some error"))
		f.logger.EXPECT().Error(mock.Anything, mock.Anything)

		// When
		resp := f.api.Get("/roles", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 500, resp.Result().StatusCode)
	})
}
