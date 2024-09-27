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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	ArgoCDNamespace       string `header:"Argocd-Namespace" required:"true" example:"argocd" doc:"The trusted namespace of the ArgoCD control plane. This should be automatically sent by Argo CD API server."`
}

func (h *ArgoCDHeaders) Application() (namespace string, name string, err error) {
	parts := strings.Split(h.ArgoCDApplicationName, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid value for %q header: expected format: <namespace>:<app-name>", "Argocd-Application-Name")
	}
	return parts[0], parts[1], nil
}

func (h ArgoCDHeaders) Groups() []string {
	return strings.Split(h.ArgoCDUserGroups, ",")
}

// GetAccessRequestInput defines the get access input parameters.
type GetAccessRequestInput struct {
	ArgoCDHeaders
	RoleName string `path:"roleName" example:"custom-role" doc:"The role name to request."`
}

// GetAccessRequestResponse defines the get access response parameters.
type GetAccessRequestResponse struct {
	Body AccessRequestResponseBody
}

// ListAccessRequestInput defines the list access input parameters.
type ListAccessRequestInput struct {
	ArgoCDHeaders
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
	RoleName string `json:"roleName" example:"custom-role" doc:"The role name to request."`
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
	RequestedAt string `json:"requestedAt,omitempty" example:"2024-02-14T18:25:50Z" doc:"The timestamp the access was requested (RFC3339 format)." format:"date-time"`
	Role        string `json:"role,omitempty" example:"DevOps" doc:"The current role the user is associated with."`
	Status      string `json:"status,omitempty" example:"GRANTED" doc:"The current access request status." enum:"REQUESTED,GRANTED,EXPIRED,DENIED"`
	ExpiresAt   string `json:"expiresAt,omitempty" example:"2024-02-14T18:25:50Z" doc:"The timestamp the access will expire (RFC3339 format)." format:"date-time"`
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

// getAccessRequestHandler is the handler implementation of the get access request operation.
func (h *APIHandler) getAccessRequestHandler(ctx context.Context, input *GetAccessRequestInput) (*GetAccessRequestResponse, error) {
	appNamespace, appName, err := input.Application()
	if err != nil {
		return nil, huma.Error400BadRequest("error getting application name", err)
	}

	key := &AccessRequestKey{
		Namespace:            input.ArgoCDNamespace,
		ApplicationName:      appName,
		ApplicationNamespace: appNamespace,
		Username:             input.ArgoCDUsername,
	}

	ar, err := h.service.GetAccessRequest(ctx, key, input.RoleName)
	if err != nil {
		h.logger.Error(err, "error getting access request")
		return nil, huma.Error500InternalServerError(fmt.Sprintf("error retrieving access request for user %s with role %s", key.Username, input.RoleName), err)
	}

	if ar == nil {
		return nil, huma.Error404NotFound("Access Request not found")
	}

	return &GetAccessRequestResponse{Body: toAccessRequestResponseBody(ar)}, nil
}

func (h *APIHandler) listAccessRequestHandler(ctx context.Context, input *ListAccessRequestInput) (*ListAccessRequestResponse, error) {
	appNamespace, appName, err := input.Application()
	if err != nil {
		return nil, huma.Error400BadRequest("error getting application name", err)
	}

	key := &AccessRequestKey{
		Namespace:            input.ArgoCDNamespace,
		ApplicationName:      appName,
		ApplicationNamespace: appNamespace,
		Username:             input.ArgoCDUsername,
	}

	accessRequests, err := h.service.ListAccessRequests(ctx, key)
	if err != nil {
		h.logger.Error(err, "error listing access request")
		return nil, huma.Error500InternalServerError(fmt.Sprintf("error listing access request for user %s", key.Username), err)
	}

	return &ListAccessRequestResponse{Body: toListAccessRequestResponseBody(accessRequests)}, nil
}

// TODO implementation
func (h *APIHandler) createAccessRequestHandler(ctx context.Context, input *CreateAccessRequestInput) (*CreateAccessRequestResponse, error) {
	// - TODO? Get current access request
	//   - Return 400/409 if already exist?

	appNamespace, appName, err := input.Application()
	if err != nil {
		return nil, huma.Error400BadRequest("error getting application name", err)
	}

	// Check if AR already exist
	key := &AccessRequestKey{
		Namespace:            input.ArgoCDNamespace,
		ApplicationName:      appName,
		ApplicationNamespace: appNamespace,
		Username:             input.ArgoCDUsername,
	}
	ar, err := h.service.GetAccessRequest(ctx, key, input.Body.RoleName)
	if err != nil {
		h.logger.Error(err, "error getting access request")
		return nil, huma.Error500InternalServerError(fmt.Sprintf("error retrieving existing access request for user %s with role %s", key.Username, input.Body.RoleName), err)
	}
	if ar != nil {
		//TODO: this only works if we expect resource to be deleted when "expired". Otherwise GetAccessRequest needs to filter only non-expired
		return nil, huma.Error409Conflict("Access Request already exist")
	}

	// Validate information in headers necessary to evaluate permissions
	app, err := h.service.GetApplication(ctx, appName, appNamespace)
	if err != nil {
		return nil, huma.Error500InternalServerError("error getting application", err)
	}
	if app == nil {
		return nil, huma.Error400BadRequest("invalid application", err)
	}
	projectName, ok, err := getApplicationProjectName(app)
	if !ok {
		return nil, huma.Error400BadRequest("invalid application spec", err)
	}
	if err != nil {
		return nil, huma.Error400BadRequest("invalid application spec", err)
	}

	project, err := h.service.GetAppProject(ctx, projectName, input.ArgoCDNamespace)
	if err != nil {
		return nil, huma.Error500InternalServerError("error getting project", err)
	}
	if project == nil {
		return nil, huma.Error400BadRequest("invalid project", err)
	}

	// Evaluate permissions
	grantingBinding, err := h.service.GetGrantingAccessBinding(ctx, input.Body.RoleName, input.ArgoCDNamespace, input.Groups(), app, project)
	if err != nil {
		return nil, huma.Error500InternalServerError("error getting access binding", err)
	}
	if grantingBinding == nil {
		return nil, huma.Error403Forbidden(fmt.Sprintf("not allowed to request role %s", input.Body.RoleName))
	}

	// Create Access Request
	ar, err = h.service.CreateAccessRequest(ctx, key, grantingBinding)
	if err != nil {
		h.logger.Error(err, "error creating accessrequest")
		return nil, huma.Error500InternalServerError(fmt.Sprintf("error creating access request for role %s", grantingBinding.Spec.RoleTemplateRef.Name), err)
	}

	return &CreateAccessRequestResponse{Body: toAccessRequestResponseBody(ar)}, nil

}

func getApplicationProjectName(app *unstructured.Unstructured) (string, bool, error) {
	return unstructured.NestedString(app.Object, "spec", "project")
}

// toAccessRequestResponseBody will convert the given ar into an AccessRequestResponseBody.
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
		Username:    ar.Spec.Subject.Username,
		Permission:  "ReadOnly",
		RequestedAt: requestedAt,
		Role:        "",
		Status:      strings.ToUpper(string(ar.Status.RequestState)),
		ExpiresAt:   expiresAt,
		Message:     "",
	}
}

func toListAccessRequestResponseBody(accessRequests []*api.AccessRequest) ListAccessRequestResponseBody {
	items := []AccessRequestResponseBody{}
	for _, ar := range accessRequests {
		items = append(items, toAccessRequestResponseBody(ar))
	}
	return ListAccessRequestResponseBody{Items: items}
}

// getAccessRequestOperation defines the get access request operation.
func getAccessRequestOperation() huma.Operation {
	return huma.Operation{
		OperationID: "get-accessrequest-by-role",
		Method:      http.MethodGet,
		Path:        "/accessrequests/{roleName}",
		Summary:     "Get AccessRequest",
		Description: "Will retrieve the access request by role for the given context",
	}
}

// listAccessRequestOperation defines the list access requests operation.
func listAccessRequestOperation() huma.Operation {
	return huma.Operation{
		OperationID: "list-accessrequest",
		Method:      http.MethodGet,
		Path:        "/accessrequests",
		Summary:     "List AccessRequests",
		Description: "Will retrieve a list of access requests for the given context",
	}
}

// createAccessRequestOperation defines the create access request operation.
func createAccessRequestOperation() huma.Operation {
	return huma.Operation{
		OperationID: "create-accessrequest",
		Method:      http.MethodPost,
		Path:        "/accessrequests",
		Summary:     "Create AccessRequest",
		Description: "Will create an access request for the given context",
	}
}

// RegisterRoutes will register all routes provided by the access request REST API
// in the given api.
func RegisterRoutes(api huma.API, h *APIHandler) {
	huma.Register(api, getAccessRequestOperation(), h.getAccessRequestHandler)
	huma.Register(api, listAccessRequestOperation(), h.listAccessRequestHandler)
	huma.Register(api, createAccessRequestOperation(), h.createAccessRequestHandler)
}
