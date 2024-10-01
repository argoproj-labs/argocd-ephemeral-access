package backend_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/argoproj-labs/ephemeral-access/internal/backend"
	"github.com/argoproj-labs/ephemeral-access/test/mocks"
	"github.com/argoproj-labs/ephemeral-access/test/utils"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func headers(username, groups, appName, projName string) []any {
	return []any{
		fmt.Sprintf("Argocd-Username: %s", username),
		fmt.Sprintf("Argocd-User-Groups: %s", groups),
		fmt.Sprintf("Argocd-Application-Name: %s", appName),
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
		arName := "some-ar"
		nsName := "some-namespace"
		username := "some-user"
		appName := "some-app"
		ar := utils.NewAccessRequest(arName, nsName, appName, "some-role", username)
		f.service.EXPECT().GetAccessRequest(mock.Anything, arName, nsName).
			Return(ar, nil)
		headers := headers(username, "group1", appName, "some-project")

		// When
		resp := f.api.Get("/accessrequests/some-ar?namespace=some-namespace", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 200, resp.Result().StatusCode)
		var respBody backend.AccessRequestResponseBody
		err := json.Unmarshal(resp.Body.Bytes(), &respBody)
		assert.NoError(t, err)
		assert.Equal(t, arName, respBody.Name)
		assert.Equal(t, username, respBody.Username)
	})
	t.Run("will return 500 on service error", func(t *testing.T) {
		// Given
		f := setup(t)
		arName := "some-ar"
		nsName := "some-namespace"
		username := "some-user"
		appName := "some-app"
		f.service.EXPECT().GetAccessRequest(mock.Anything, arName, nsName).
			Return(nil, fmt.Errorf("some-error"))
		f.logger.EXPECT().Error(mock.Anything, mock.Anything)
		headers := headers(username, "group1", appName, "some-project")

		// When
		resp := f.api.Get("/accessrequests/some-ar?namespace=some-namespace", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 500, resp.Result().StatusCode)
	})
	t.Run("will return 404 if access request not found", func(t *testing.T) {
		// Given
		f := setup(t)
		arName := "some-ar"
		nsName := "some-namespace"
		username := "some-user"
		appName := "some-app"
		f.service.EXPECT().GetAccessRequest(mock.Anything, arName, nsName).
			Return(nil, nil)
		headers := headers(username, "group1", appName, "some-project")

		// When
		resp := f.api.Get("/accessrequests/some-ar?namespace=some-namespace", headers...)

		// Then
		assert.NotNil(t, resp)
		assert.Equal(t, 404, resp.Result().StatusCode)
	})

}
