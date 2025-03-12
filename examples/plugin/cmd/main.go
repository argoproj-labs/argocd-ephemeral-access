package main

import (
	"fmt"

	argocd "github.com/argoproj-labs/argocd-ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/hashicorp/go-hclog"

	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/log"
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/plugin"
	goPlugin "github.com/hashicorp/go-plugin"
)

// SomePlugin this is the struct that implements the plugin.AccessRequester
// interface. It is preferable (but not required) to rename this struct to
// something related to what your plugin will be doing. Example: JiraPlugin
type SomePlugin struct {
	Logger hclog.Logger
}

// This function will be called once when the EphemeralAccess controller is
// initialized. It can be used to instantiate clients to other services needed by
// this plugin for example. Those instances can then be assigned in this p for
// later use. Just return nil of this is not required for your plugin.
func (p *SomePlugin) Init() error {
	p.Logger.Info("This is a call to the Init method")
	return nil
}

// GrantAccess is the method that will be called by the EphemeralAccess controller
// when an AccessRequest is created. The EphemeralAccess controller will only proceed
// granting the access if the returned GrantResponse.Status is plugin.GrantStatusGranted.
// Returning a nil GrantResponse will cause an error in the EphemeralAccess controller
// and no access will be granted.
// This function can be used to addresss different use-cases.
// A few examples are:
// - verify if the given app has an associated Change Request in approved state
// - access internal service for last mile user access validation
func (p *SomePlugin) GrantAccess(ar *api.AccessRequest, app *argocd.Application) (*plugin.GrantResponse, error) {
	p.Logger.Info("This is a call to the GrantAccess method")
	return &plugin.GrantResponse{
		Status:  plugin.GrantStatusGranted,
		Message: "Granted access by the example plugin",
	}, nil
}

// RevokeAccess is the method that will be called by the EphemeralAccess controller
// when an AccessRequest is expired. Plugins authors may decide to not implement this
// method depending on the use case. In this case it is safe to just return nil, nil.
func (p *SomePlugin) RevokeAccess(ar *api.AccessRequest, app *argocd.Application) (*plugin.RevokeResponse, error) {
	p.Logger.Info("This is a call to the RevokeAccess method")
	return &plugin.RevokeResponse{
		Status:  plugin.RevokeStatusRevoked,
		Message: "Revoked access by the example plugin",
	}, nil
}

// main must be defined as it is the plugin entrypoint. It will be automatically called
// by the EphemeralAccess controller.
func main() {
	// NewPluginLogger will return a logger that will respect the same level and format
	// defined to the EphemeralAccess controller.
	logger, err := log.NewPluginLogger()
	if err != nil {
		panic(fmt.Sprintf("Error creating plugin logger: %s", err))
	}

	// create a new instance of your plugin after initializing the logger and other
	// dependencies. However it is preferable to leave the main function lean and
	// initialize plugin dependencies in the `Init` method.
	p := &SomePlugin{
		Logger: logger,
	}

	// create the plugin server config
	srvConfig := plugin.NewServerConfig(p, logger)
	// initialize the plugin server
	goPlugin.Serve(srvConfig)
}
