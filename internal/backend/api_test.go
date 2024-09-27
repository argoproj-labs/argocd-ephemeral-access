package backend_test

import (
	"encoding/json"
	"fmt"
	"testing"

	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/argoproj-labs/ephemeral-access/test/mocks"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newAccessRequest(key *backend.AccessRequestKey, roleName string) *api.AccessRequest {
	return &api.AccessRequest{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AccessRequest",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.ResourceName(roleName),
			Namespace: key.Namespace,
		},
		Spec: api.AccessRequestSpec{
			Duration:         metav1.Duration{},
			RoleTemplateName: roleName,
			Application: api.TargetApplication{
				Name:      key.ApplicationName,
				Namespace: key.ApplicationNamespace,
			},
			Subject: api.Subject{
				Username: key.Username,
			},
		},
	}
}

func headers(namespace, username, groups, appNs, appName, projName string) []any {
	return []any{
		fmt.Sprintf("Argocd-Namespace: %s", namespace),
		fmt.Sprintf("Argocd-Username: %s", username),
		fmt.Sprintf("Argocd-User-Groups: %s", groups),
		fmt.Sprintf("Argocd-Application-Name: %s:%s", appNs, appName),
		fmt.Sprintf("Argocd-Project-Name: %s", projName),
	}
}

func TestGetAccessRequest(t *testing.T) {
	type fixture struct {
		api     humatest.TestAPI
		service *mocks.MockService
		logger  *mocks.MockLogger
	}
	setup := func(t *testing.T) *fixture {
		_, api := humatest.New(t)
		service := mocks.NewMockService(t)
		logger := mocks.NewMockLogger(t)
		handler := backend.NewAPIHandler(service, logger)
		backend.RegisterRoutes(api, handler)
		return &fixture{
			api:     api,
			service: service,
			logger:  logger,
		}
	}
	t.Run("will return access request successfully", func(t *testing.T) {
		// Given
		f := setup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		headers := headers(key.Namespace, key.Username, "group1", key.ApplicationNamespace, key.ApplicationName, "some-project")
		roleName := "some-role"
		ar := newAccessRequest(key, roleName)
		f.service.EXPECT().GetAccessRequest(mock.Anything, key, roleName).Return(ar, nil)

		// When
		resp := f.api.Get(fmt.Sprintf("/accessrequests/%s", roleName), headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 200, resp.Result().StatusCode)
		var respBody backend.AccessRequestResponseBody
		err := json.Unmarshal(resp.Body.Bytes(), &respBody)
		assert.NoError(t, err)
		assert.Equal(t, ar.GetName(), respBody.Name)
		assert.Equal(t, ar.Spec.Subject.Username, respBody.Username)
	})
	t.Run("will return 500 on service error", func(t *testing.T) {
		// Given
		f := setup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		roleName := "some-role"
		headers := headers(key.Namespace, key.Username, "group1", key.ApplicationNamespace, key.ApplicationName, "some-project")
		f.service.EXPECT().GetAccessRequest(mock.Anything, key, roleName).Return(nil, fmt.Errorf("some-error"))
		f.logger.EXPECT().Error(mock.Anything, mock.Anything)

		// When
		resp := f.api.Get(fmt.Sprintf("/accessrequests/%s", roleName), headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 500, resp.Result().StatusCode)
	})
	t.Run("will return 404 if access request not found", func(t *testing.T) {
		// Given
		f := setup(t)
		key := &backend.AccessRequestKey{
			Namespace:            "some-namespace",
			ApplicationName:      "some-app",
			ApplicationNamespace: "app-ns",
			Username:             "some-user",
		}
		roleName := "some-role"
		headers := headers(key.Namespace, key.Username, "group1", key.ApplicationNamespace, key.ApplicationName, "some-project")
		f.service.EXPECT().GetAccessRequest(mock.Anything, key, roleName).Return(nil, nil)

		// When
		resp := f.api.Get(fmt.Sprintf("/accessrequests/%s", roleName), headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 404, resp.Result().StatusCode)
	})

}
