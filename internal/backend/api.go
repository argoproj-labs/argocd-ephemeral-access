package backend

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	api "github.com/argoproj-labs/ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	"github.com/danielgtaylor/huma/v2"
)

const (
	// APITitle refers to the API description used in the open-api spec.
	APITitle = "Ephemeral Access API"
	// APIVersion refers to the API version used in the open-api spec.
	APIVersion = "0.0.1"
)

// ArgoCDHeaders defines the required headers that are sent by Argo CD
// API server to proxy extensions.
type ArgoCDHeaders struct {
	ArgoCDUsername        string `header:"Argocd-Username" required:"true" example:"some-user@acme.org" doc:"The trusted ArgoCD username header. This should be automatically sent by Argo CD API server."`
	ArgoCDUserGroups      string `header:"Argocd-User-Groups" required:"true" example:"group1,group2" doc:"The trusted ArgoCD user groups header. This should be automatically sent by Argo CD API server."`
	ArgoCDApplicationName string `header:"Argocd-Application-Name" required:"true" example:"some-namespace:app-name" doc:"The trusted ArgoCD application header. This should be automatically sent by Argo CD API server."`
	ArgoCDProjectName     string `header:"Argocd-Project-Name" required:"true" example:"some-project-name" doc:"The trusted ArgoCD project header. This should be automatically sent by Argo CD API server."`
}

// GetAccessRequestInput defines the get access input parameters.
type GetAccessRequestInput struct {
	ArgoCDHeaders
	Name      string `path:"name" example:"some-name" doc:"The access request name."`
	Namespace string `query:"namespace" example:"some-namespace" doc:"The namespace to use while searching for the access request."`
}

// GetAccessRequestResponse defines the get access response parameters.
type GetAccessRequestResponse struct {
	Body AccessRequestResponseBody
}

// ListAccessRequestInput defines the list access input parameters.
type ListAccessRequestInput struct {
	ArgoCDHeaders
	Username string `query:"username" example:"some-user@acme.org" doc:"Will search for all access requests for the given username."`
	AppName  string `query:"appName" example:"namespace:some-app-name" doc:"Will search for all access requests for the given application (format <namespace>:<name>)."`
}

// ListAccessRequestResponse defines the list access response parameters.
type ListAccessRequestResponse struct {
	Body ListAccessRequestResponseBody
}

// ListAccessRequestResponseBody defines the list access response body.
type ListAccessRequestResponseBody struct {
	Items []AccessRequestResponseBody `json:"items"`
}

// CreateAccessRequestInput defines the create access input parameters.
type CreateAccessRequestInput struct {
	ArgoCDHeaders
	Body CreateAccessRequestBody
}

// CreateAccessRequestBody defines the create access response body.
type CreateAccessRequestBody struct {
	Username    string `json:"username" example:"some-user@acme.org" doc:"The user to be associated with the access request."`
	Application string `json:"appName" example:"some-namespace:app-name" doc:"The application to be associated with the access request (format <namespace>:<name>)."`
}

// CreateAccessRequestResponse defines the create access response.
type CreateAccessRequestResponse struct {
	Body AccessRequestResponseBody
}

// AccessRequestResponseBody defines the access request fields returned as part of
// the response body.
type AccessRequestResponseBody struct {
	Name        string `json:"name" example:"some-accessrequest" doc:"The access request name."`
	Namespace   string `json:"namespace" example:"some-namespace" doc:"The access request namespace."`
	Username    string `json:"username" example:"some-user@acme.org" doc:"The user associated with the access request."`
	Permission  string `json:"permission" example:"ReadOnly" doc:"The current permission description for the user."`
	RequestedAt string `json:"requestedAt,omitempty" format:"date-time" example:"2024-02-14T18:25:50Z" doc:"The timestamp the access was requested (RFC3339 format)."`
	Role        string `json:"role,omitempty" example:"DevOps" doc:"The current role the user is associated with."`
	Status      string `json:"status,omitempty" enum:"REQUESTED,GRANTED,EXPIRED,DENIED" example:"GRANTED" doc:"The current access request status."`
	ExpiresAt   string `json:"expiresAt,omitempty" format:"date-time" example:"2024-02-14T18:25:50Z" doc:"The timestamp the access will expire (RFC3339 format)."`
	Message     string `json:"message,omitempty" example:"Click the link to see more details: ..." doc:"A human readeable description with details about the access request."`
}

// APIHandler is responsible for defining all handlers available as part of the
// AccessRequest REST API.
type APIHandler struct {
	service Service
	logger  log.Logger
}

// NewAPIHandler will instantiate and return a new APIHandler.
func NewAPIHandler(s Service, logger log.Logger) *APIHandler {
	return &APIHandler{
		service: s,
		logger:  logger,
	}
}

// getAccessRequestHandler is the handler implementation of the get access request
// operation.
func (h *APIHandler) getAccessRequestHandler(ctx context.Context, input *GetAccessRequestInput) (*GetAccessRequestResponse, error) {
	ar, err := h.service.GetAccessRequest(ctx, input.Name, input.Namespace)
	if err != nil {
		h.logger.Error(err, "error getting accessrequest")
		return nil, huma.Error500InternalServerError(fmt.Sprintf("error retrieving access request for %s/%s", input.Namespace, input.Name), err)
	}

	if ar == nil {
		return nil, huma.Error404NotFound(fmt.Sprintf("AccessRequest %s/%s not found", input.Namespace, input.Name))
	}

	return &GetAccessRequestResponse{
		Body: toAccessRequestResponseBody(ar),
	}, nil
}

// TODO implementation
func (h *APIHandler) listAccessRequestHandler(ctx context.Context, input *ListAccessRequestInput) (*ListAccessRequestResponse, error) {
	return nil, huma.Error501NotImplemented("not implemented")
}

// TODO implementation
func (h *APIHandler) createAccessRequestHandler(ctx context.Context, input *CreateAccessRequestInput) (*CreateAccessRequestResponse, error) {
	return nil, huma.Error501NotImplemented("not implemented")
}

// toAccessRequestResponseBody will convert the given ar into an
// AccessRequestResponseBody.
func toAccessRequestResponseBody(ar *api.AccessRequest) AccessRequestResponseBody {
	expiresAt := ""
	if ar.Status.ExpiresAt != nil {
		expiresAt = ar.Status.ExpiresAt.Format(time.RFC3339)
	}
	requestedAt := ""
	if len(ar.Status.History) > 0 {
		requestedAt = ar.Status.History[0].TransitionTime.Format(time.RFC3339)
	}

	return AccessRequestResponseBody{
		Name:        ar.GetName(),
		Namespace:   ar.GetNamespace(),
		Username:    ar.Spec.Subjects[0].Username,
		Permission:  "ReadOnly",
		RequestedAt: requestedAt,
		Role:        "",
		Status:      strings.ToUpper(string(ar.Status.RequestState)),
		ExpiresAt:   expiresAt,
		Message:     "",
	}
}

// getAccessRequestOperation defines the get access request operation.
func getAccessRequestOperation() huma.Operation {
	return huma.Operation{
		OperationID: "get-accessrequest-by-name",
		Method:      http.MethodGet,
		Path:        "/accessrequests/{name}",
		Summary:     "Get AccessRequest",
		Description: "Will retrieve the accessrequest by name",
	}
}

// listAccessRequestOperation defines the list access requests operation.
func listAccessRequestOperation() huma.Operation {
	return huma.Operation{
		OperationID: "list-accessrequest",
		Method:      http.MethodGet,
		Path:        "/accessrequests",
		Summary:     "List AccessRequests",
		Description: "Will retrieve a list of accessrequests respecting the search criteria provided as query params.",
	}
}

// createAccessRequestOperation defines the create access request operation.
func createAccessRequestOperation() huma.Operation {
	return huma.Operation{
		OperationID: "create-accessrequest",
		Method:      http.MethodPost,
		Path:        "/accessrequests",
		Summary:     "Create AccessRequest",
		Description: "Will create an access request for the given user and application.",
	}
}

// RegisterRoutes will register all routes provided by the access request REST API
// in the given api.
func RegisterRoutes(api huma.API, h *APIHandler) {
	huma.Register(api, getAccessRequestOperation(), h.getAccessRequestHandler)
	huma.Register(api, listAccessRequestOperation(), h.listAccessRequestHandler)
	huma.Register(api, createAccessRequestOperation(), h.createAccessRequestHandler)
}
