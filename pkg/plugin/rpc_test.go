package plugin_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	argocd "github.com/argoproj-labs/ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/pkg/plugin"
	"github.com/argoproj-labs/ephemeral-access/test/mocks"
	goPlugin "github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fixture struct {
	accessRequesterMock *mocks.MockAccessRequester
	cancel              func()
	client              plugin.AccessRequester
}

func newFixture(t *testing.T) *fixture {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan *goPlugin.ReattachConfig, 1)

	mock := mocks.NewMockAccessRequester(t)
	srvConfig := plugin.NewServerConfig(mock, nil)
	srvConfig.Test = &goPlugin.ServeTestConfig{
		Context:          ctx,
		ReattachConfigCh: ch,
	}

	go goPlugin.Serve(srvConfig)

	// We should get a config
	var config *goPlugin.ReattachConfig
	select {
	case config = <-ch:
	case <-time.After(2000 * time.Millisecond):
		t.Fatal("ReattachConfig not received: timed out!")
	}
	if config == nil {
		t.Fatal("config should not be nil")
	}

	cliConfig := plugin.NewClientConfig("", nil)
	cliConfig.Cmd = nil
	cliConfig.Reattach = config
	client := goPlugin.NewClient(cliConfig)

	plugin, err := plugin.GetAccessRequester(client)
	if err != nil {
		t.Fatalf("error getting AccessRequester: %s", err)
	}
	return &fixture{
		accessRequesterMock: mock,
		cancel:              cancel,
		client:              plugin,
	}
}

func TestAccessRequesterRPC(t *testing.T) {
	newAccessRequest := func(name, namespace, roletemplate, username string) *api.AccessRequest {
		return &api.AccessRequest{
			Spec: api.AccessRequestSpec{
				Role: api.TargetRole{
					TemplateRef: api.TargetRoleTemplate{
						Name:      roletemplate,
						Namespace: "ephemeral",
					},
				},
				Application: api.TargetApplication{
					Name:      name,
					Namespace: namespace,
				},
				Subject: api.Subject{
					Username: username,
				},
			},
		}
	}
	newApplication := func(project string) *argocd.Application {
		return &argocd.Application{
			Spec: argocd.ApplicationSpec{
				Project: project,
			},
		}
	}
	t.Run("will validate Init is invoked without errors", func(t *testing.T) {
		// Given
		f := newFixture(t)
		defer f.cancel()
		f.accessRequesterMock.EXPECT().Init().Return(nil)

		// When
		err := f.client.Init()

		// Then
		assert.NoError(t, err)
		f.accessRequesterMock.AssertNumberOfCalls(t, "Init", 1)
	})
	t.Run("will validate Init is invoked returning error", func(t *testing.T) {
		// Given
		f := newFixture(t)
		defer f.cancel()
		expectedError := fmt.Errorf("Init error")
		f.accessRequesterMock.EXPECT().Init().Return(expectedError)

		// When
		err := f.client.Init()

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), expectedError.Error())
		f.accessRequesterMock.AssertNumberOfCalls(t, "Init", 1)
		f.accessRequesterMock.AssertNumberOfCalls(t, "GrantAccess", 0)
		f.accessRequesterMock.AssertNumberOfCalls(t, "RevokeAccess", 0)
	})
	t.Run("will validate GrantAccess is invoked without errors", func(t *testing.T) {
		// Given
		f := newFixture(t)
		defer f.cancel()
		ar := newAccessRequest("some-ar", "some-ns", "some-roletmpl", "some-user")
		app := newApplication("some-project")
		var receivedAr *api.AccessRequest
		var receivedApp *argocd.Application
		expectedMessage := "some grant message"
		runFn := func(ar *api.AccessRequest, app *argocd.Application) (*plugin.GrantResponse, error) {
			receivedAr = ar
			receivedApp = app
			return &plugin.GrantResponse{
				Status:  plugin.Granted,
				Message: expectedMessage,
			}, nil

		}
		f.accessRequesterMock.EXPECT().GrantAccess(ar, app).
			RunAndReturn(runFn)

		// When
		resp, err := f.client.GrantAccess(ar, app)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, ar, receivedAr)
		assert.Equal(t, app, receivedApp)
		assert.Equal(t, plugin.Granted, resp.Status)
		assert.Equal(t, expectedMessage, resp.Message)
		f.accessRequesterMock.AssertNumberOfCalls(t, "Init", 0)
		f.accessRequesterMock.AssertNumberOfCalls(t, "GrantAccess", 1)
		f.accessRequesterMock.AssertNumberOfCalls(t, "RevokeAccess", 0)
	})
	t.Run("will validate GrantAccess properly returns error", func(t *testing.T) {
		// Given
		f := newFixture(t)
		defer f.cancel()
		expectedErr := "grant access error"
		f.accessRequesterMock.EXPECT().GrantAccess(mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf(expectedErr))

		// When
		resp, err := f.client.GrantAccess(nil, nil)

		// Then
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, expectedErr, err.Error())
		f.accessRequesterMock.AssertNumberOfCalls(t, "Init", 0)
		f.accessRequesterMock.AssertNumberOfCalls(t, "GrantAccess", 1)
		f.accessRequesterMock.AssertNumberOfCalls(t, "RevokeAccess", 0)
	})
	t.Run("will validate RevokeAccess is invoked without errors", func(t *testing.T) {
		// Given
		f := newFixture(t)
		defer f.cancel()
		ar := newAccessRequest("some-ar", "some-ns", "some-roletmpl", "some-user")
		app := newApplication("some-project")
		var receivedAr *api.AccessRequest
		var receivedApp *argocd.Application
		expectedMessage := "some revoke message"
		runFn := func(ar *api.AccessRequest, app *argocd.Application) (*plugin.RevokeResponse, error) {
			receivedAr = ar
			receivedApp = app
			return &plugin.RevokeResponse{
				Status:  plugin.Revoked,
				Message: expectedMessage,
			}, nil

		}
		f.accessRequesterMock.EXPECT().RevokeAccess(ar, app).
			RunAndReturn(runFn)

		// When
		resp, err := f.client.RevokeAccess(ar, app)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, ar, receivedAr)
		assert.Equal(t, app, receivedApp)
		assert.Equal(t, plugin.Revoked, resp.Status)
		assert.Equal(t, expectedMessage, resp.Message)
		f.accessRequesterMock.AssertNumberOfCalls(t, "Init", 0)
		f.accessRequesterMock.AssertNumberOfCalls(t, "GrantAccess", 0)
		f.accessRequesterMock.AssertNumberOfCalls(t, "RevokeAccess", 1)
	})
	t.Run("will validate RevokeAccess properly returns error", func(t *testing.T) {
		// Given
		f := newFixture(t)
		defer f.cancel()
		expectedErr := "revoke access error"
		f.accessRequesterMock.EXPECT().RevokeAccess(mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf(expectedErr))

		// When
		resp, err := f.client.RevokeAccess(nil, nil)

		// Then
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, expectedErr, err.Error())
		f.accessRequesterMock.AssertNumberOfCalls(t, "Init", 0)
		f.accessRequesterMock.AssertNumberOfCalls(t, "GrantAccess", 0)
		f.accessRequesterMock.AssertNumberOfCalls(t, "RevokeAccess", 1)
	})
}
