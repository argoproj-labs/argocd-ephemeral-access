package plugin

import (
	"encoding/gob"
	"fmt"
	"net/rpc"
	"os/exec"

	argocd "github.com/argoproj-labs/ephemeral-access/api/argoproj/v1alpha1"
	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"

	"github.com/hashicorp/go-hclog"
	goPlugin "github.com/hashicorp/go-plugin"
)

type GrantStatus string
type RevokeStatus string

const (
	Granted       GrantStatus  = "granted"
	GrantPending  GrantStatus  = "grant-pending"
	Denied        GrantStatus  = "denied"
	Revoked       RevokeStatus = "revoked"
	RevokePending RevokeStatus = "revoke-pending"
	Key           string       = "ephemeralaccess"
)

func init() {
	gob.Register(&PluginError{})
}

// PluginError the error type returned by the rpc server over the wire
type PluginError struct {
	Err string
}

// Error the error implementation
func (pe *PluginError) Error() string {
	return pe.Err
}

// AccessRequester defines the main interface that should be implemented by
// ephemeral access plugins.
type AccessRequester interface {
	Init() error
	GrantAccess(ar *api.AccessRequest, app *argocd.Application) (*GrantResponse, error)
	RevokeAccess(ar *api.AccessRequest, app *argocd.Application) (*RevokeResponse, error)
}

// GrantResponse defines the response that will be returned by access
// request plugins.
type GrantResponse struct {
	Status  GrantStatus
	Message string
}

// RevokeResponse defines the response that will be returned by access
// request plugins.
type RevokeResponse struct {
	Status  RevokeStatus
	Message string
}

// GrantAccessArgsRPC wraps the args that are sent to the GrantAccess function
// over RPC.
type GrantAccessArgsRPC struct {
	AccReq *api.AccessRequest
	App    *argocd.Application
}

// RevokeAccessArgsRPC wraps the args that are sent to the RevokeAccess function
// over RPC.
type RevokeAccessArgsRPC struct {
	AccReq *api.AccessRequest
	App    *argocd.Application
}

// InitResponseRPC wraps the response that are received by the Init function over
// RPC.
type InitResponseRPC struct {
	Err error
}

// GrantAccessResponseRPC wraps the response that are received by the GrantAccess
// function over RPC.
type GrantAccessResponseRPC struct {
	Response *GrantResponse
	Err      error
}

// RevokeAccessResponseRPC wraps the response that are received by the GrantAccess
// function over RPC.
type RevokeAccessResponseRPC struct {
	Response *RevokeResponse
	Err      error
}

// AccessRequesterRPCServer is the server side stub used by AccessRequester
// plugins.
type AccessRequesterRPCServer struct {
	Impl AccessRequester
}

// Init is the server side stub implementation of the Init function.
func (s *AccessRequesterRPCServer) Init(args any, resp *InitResponseRPC) error {
	err := s.Impl.Init()
	if err != nil {
		resp.Err = &PluginError{
			Err: err.Error(),
		}
	}
	return nil
}

// GrantAccess is the server side stub implementation of the GrantAccess function.
func (s *AccessRequesterRPCServer) GrantAccess(args GrantAccessArgsRPC, resp *GrantAccessResponseRPC) error {
	gr, err := s.Impl.GrantAccess(args.AccReq, args.App)
	resp.Response = gr
	if err != nil {
		resp.Err = &PluginError{
			Err: err.Error(),
		}
	}
	return nil
}

// RevokeAccess is the server side stub implementation of the RevokeAccess function.
func (s *AccessRequesterRPCServer) RevokeAccess(args RevokeAccessArgsRPC, resp *RevokeAccessResponseRPC) error {
	rr, err := s.Impl.RevokeAccess(args.AccReq, args.App)
	resp.Response = rr
	if err != nil {
		resp.Err = &PluginError{
			Err: err.Error(),
		}
	}
	return nil
}

// AccessRequesterRPCClient is the client side stub used by AccessRequester
// plugins.
type AccessRequesterRPCClient struct {
	client *rpc.Client
}

// Init is the client side stub implementation of the Init function.
func (c *AccessRequesterRPCClient) Init() error {
	resp := InitResponseRPC{}
	err := c.client.Call("Plugin.Init", new(interface{}), &resp)
	if err != nil {
		return fmt.Errorf("Init RPC call error: %s", err)
	}
	return resp.Err
}

// GrantAccess is the client side stub implementation of the GrantAccess function.
func (c *AccessRequesterRPCClient) GrantAccess(ar *api.AccessRequest, app *argocd.Application) (*GrantResponse, error) {
	resp := GrantAccessResponseRPC{}
	args := GrantAccessArgsRPC{
		AccReq: ar,
		App:    app,
	}
	err := c.client.Call("Plugin.GrantAccess", &args, &resp)
	if err != nil {
		return nil, fmt.Errorf("GrantAccess RPC call error: %s", err)
	}
	return resp.Response, resp.Err
}

// RevokeAccess is the client side stub implementation of the RevokeAccess function.
func (c *AccessRequesterRPCClient) RevokeAccess(ar *api.AccessRequest, app *argocd.Application) (*RevokeResponse, error) {
	resp := RevokeAccessResponseRPC{}
	args := RevokeAccessArgsRPC{
		AccReq: ar,
		App:    app,
	}
	err := c.client.Call("Plugin.RevokeAccess", &args, &resp)
	if err != nil {
		return nil, fmt.Errorf("RevokeAccess RPC call error: %s", err)
	}
	return resp.Response, resp.Err
}

// AccessRequestPlugin is the implementation of plugin.Plugin so we can serve/consume
//
// This has two methods:
// - Server(): must return an RPC server for this plugin type.
// - Client(): must return an implementation of the interface that communicates
// over an RPC client.
type AccessRequestPlugin struct {
	Impl AccessRequester
}

// Server will build and return the server side stub for AccessRequester plugings.
func (p *AccessRequestPlugin) Server(*goPlugin.MuxBroker) (interface{}, error) {
	return &AccessRequesterRPCServer{Impl: p.Impl}, nil
}

// Client will build and return the client side stub for AccessRequester plugings.
func (AccessRequestPlugin) Client(b *goPlugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &AccessRequesterRPCClient{client: c}, nil
}

// handshake returns the handshake config used by AccessRequester plugins.
func handshake() goPlugin.HandshakeConfig {
	return goPlugin.HandshakeConfig{
		ProtocolVersion:  1,
		MagicCookieKey:   "EPHEMERAL_ACCESS_PLUGIN",
		MagicCookieValue: "ephemeralaccess",
	}
}

// NewServerConfig will build and return a new instance of server stub configs.
func NewServerConfig(impl AccessRequester, log hclog.Logger) *goPlugin.ServeConfig {
	pluginMap := map[string]goPlugin.Plugin{
		Key: &AccessRequestPlugin{
			Impl: impl,
		},
	}
	return &goPlugin.ServeConfig{
		HandshakeConfig: handshake(),
		Plugins:         pluginMap,
		Logger:          log,
	}
}

// NewClientConfig will build and return a new instance of client stub configs.
func NewClientConfig(pluginPath string, log hclog.Logger) *goPlugin.ClientConfig {
	pluginMap := map[string]goPlugin.Plugin{
		Key: &AccessRequestPlugin{},
	}
	return &goPlugin.ClientConfig{
		HandshakeConfig: handshake(),
		Plugins:         pluginMap,
		Cmd:             exec.Command(pluginPath),
		Logger:          log,
	}
}

// GetAccessRequester will attempt to instantiate a new AccessRequester from the
// provided client. The returned AccessRequester will invoke RPC calls targeting
// the plugin implementation on method calls.
func GetAccessRequester(client *goPlugin.Client) (AccessRequester, error) {
	rpcClient, err := client.Client()
	if err != nil {
		return nil, fmt.Errorf("error retrieving rpc client: %w", err)
	}

	raw, err := rpcClient.Dispense(Key)
	if err != nil {
		return nil, fmt.Errorf("error getting a new plugin instance: %w", err)
	}

	plugin, ok := raw.(AccessRequester)
	if !ok {
		return nil, fmt.Errorf("returned plugin instance is not AccessRequester")
	}

	return plugin, nil
}
