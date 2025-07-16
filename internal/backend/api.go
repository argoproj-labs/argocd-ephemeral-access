package backend

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	api "github.com/argoproj-labs/argocd-ephemeral-access/api/ephemeral-access/v1alpha1"
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/log"
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
	ArgoCDNamespace       string `header:"Argocd-Namespace" required:"true" example:"argocd" doc:"The trusted namespace of the ArgoCD control plane. This should be automatically sent by Argo CD API server."`
}

func (h *ArgoCDHeaders) Application() (namespace string, name string, err error) {
	parts := strings.Split(h.ArgoCDApplicationName, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid value for %q header: expected format: <namespace>:<app-name>", "Argocd-Application-Name")
	}
	if parts[0] == "" {
		return "", "", fmt.Errorf("invalid value for %q header: application namespace must be informed", "Argocd-Application-Name")
	}
	if parts[1] == "" {
		return "", "", fmt.Errorf("invalid value for %q header: application name must be informed", "Argocd-Application-Name")
	}
	return parts[0], parts[1], nil
}

func (h *ArgoCDHeaders) Groups() []string {
	return strings.Split(h.ArgoCDUserGroups, ",")
}

// ListAccessRequestInput defines the list access input parameters.
type ListAccessRequestInput struct {
	ArgoCDHeaders
}

// ListAllowedRolesInput defines the input parameters list of allowed roles.
type ListAllowedRolesInput struct {
	ArgoCDHeaders
}

// ListAllowedRolesResponse defines the response of allowed roles requests.
type ListAllowedRolesResponse struct {
	Body ListAllowedRolesResponseBody
}

// ListAllowedRolesResponseBody defines the response body of allowed roles requests.
type ListAllowedRolesResponseBody struct {
	Items []AllowedRoleResponseBody `json:"items"`
}

// AllowedRoleResponseBody defines the allowed role response.
type AllowedRoleResponseBody struct {
	RoleName        string `json:"roleName" example:"custom-role-template" doc:"The role template name to request."`
	RoleDisplayName string `json:"roleDisplayName" example:"Write (DevOps)" doc:"The human friendly name of the role that can be used to display to users."`
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
	RoleName string `json:"roleName" example:"custom-role-template" doc:"The role template name to request."`
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
	Permission  string `json:"permission" example:"Operator Access" doc:"The permission description of the role associated to this access request."`
	Role        string `json:"role" example:"custom-role-template" doc:"The role template associated to this access request."`
	RequestedAt string `json:"requestedAt,omitempty" example:"2024-02-14T18:25:50Z" doc:"The timestamp the access was requested (RFC3339 format)." format:"date-time"`
	Status      string `json:"status,omitempty" example:"GRANTED" doc:"The current access request status." enum:"INITIATED,REQUESTED,GRANTED,EXPIRED,DENIED,INVALID"`
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

func (h *APIHandler) listAllowedRolesHandler(ctx context.Context, input *ListAllowedRolesInput) (*ListAllowedRolesResponse, error) {
	if input.ArgoCDUserGroups == "" {
		return nil, huma.Error400BadRequest(fmt.Sprintf("user (%s) has no groups", input.ArgoCDUsername))
	}
	groups := strings.Split(input.ArgoCDUserGroups, ",")
	appNamespace, appName, err := input.Application()
	if err != nil {
		return nil, huma.Error400BadRequest("error getting application name", err)
	}
	// Validate information in headers necessary to evaluate permissions
	app, err := h.service.GetApplication(ctx, appName, appNamespace)
	if err != nil {
		return nil, h.loggedError(huma.Error500InternalServerError("error getting application", err))
	}
	if app == nil {
		return nil, huma.Error404NotFound("Argo CD Application not found")
	}

	project, err := h.service.GetAppProject(ctx, input.ArgoCDProjectName, input.ArgoCDNamespace)
	if err != nil {
		return nil, h.loggedError(huma.Error500InternalServerError("error getting project", err))
	}
	if project == nil {
		return nil, huma.Error404NotFound("Argo CD AppProject not found")
	}

	abList, err := h.service.GetAccessBindingsForGroups(ctx, input.ArgoCDNamespace, groups, app, project)
	if err != nil {
		return nil, h.loggedError(huma.Error500InternalServerError(fmt.Sprintf("error listing allowed roles for user %s", input.ArgoCDUsername), err))
	}

	return &ListAllowedRolesResponse{Body: toListAllowedRolesResponseBody(abList)}, nil
}

func toListAllowedRolesResponseBody(abList []*api.AccessBinding) ListAllowedRolesResponseBody {
	result := ListAllowedRolesResponseBody{}
	for _, ab := range abList {
		// if the friendly name is nil it will fall back to display
		// the roletemplate name
		displayName := ab.Spec.RoleTemplateRef.Name
		if ab.Spec.FriendlyName != nil {
			displayName = *ab.Spec.FriendlyName
		}
		item := AllowedRoleResponseBody{
			RoleName:        ab.Spec.RoleTemplateRef.Name,
			RoleDisplayName: displayName,
		}
		result.Items = append(result.Items, item)
	}
	return result
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

	accessRequests, err := h.service.ListAccessRequests(ctx, key, true)
	if err != nil {
		return nil, h.loggedError(huma.Error500InternalServerError(fmt.Sprintf("error listing access request for user %s", key.Username), err))
	}

	return &ListAccessRequestResponse{Body: toListAccessRequestResponseBody(accessRequests)}, nil
}

func (h *APIHandler) createAccessRequestHandler(ctx context.Context, input *CreateAccessRequestInput) (*CreateAccessRequestResponse, error) {
	appNamespace, appName, err := input.Application()
	if err != nil {
		return nil, huma.Error400BadRequest("invalid application", err)
	}

	// Check if AR already exist
	key := &AccessRequestKey{
		Namespace:            input.ArgoCDNamespace,
		ApplicationName:      appName,
		ApplicationNamespace: appNamespace,
		Username:             input.ArgoCDUsername,
	}
	ar, err := h.service.GetAccessRequestByRole(ctx, key, input.Body.RoleName)
	if err != nil {
		return nil, h.loggedError(huma.Error500InternalServerError(fmt.Sprintf("error retrieving existing access request for user %s with role %s", key.Username, input.Body.RoleName), err))
	}
	if ar != nil {
		return nil, huma.Error409Conflict("AccessRequest already exists")
	}

	// Validate information in headers necessary to evaluate permissions
	app, err := h.service.GetApplication(ctx, appName, appNamespace)
	if err != nil {
		return nil, h.loggedError(huma.Error500InternalServerError("error getting application", err))
	}
	if app == nil {
		return nil, huma.Error400BadRequest("invalid application", err)
	}

	project, err := h.service.GetAppProject(ctx, input.ArgoCDProjectName, input.ArgoCDNamespace)
	if err != nil {
		return nil, h.loggedError(huma.Error500InternalServerError("error getting project", err))
	}
	if project == nil {
		return nil, huma.Error400BadRequest("invalid project", err)
	}

	// Evaluate permissions
	grantingBinding, err := h.service.GetGrantingAccessBinding(ctx, input.Body.RoleName, input.ArgoCDNamespace, input.Groups(), app, project)
	if err != nil {
		return nil, h.loggedError(huma.Error500InternalServerError("error getting access binding", err))
	}
	if grantingBinding == nil {
		return nil, huma.Error403Forbidden(fmt.Sprintf("not allowed to request role %s", input.Body.RoleName))
	}

	// Create Access Request
	ar, err = h.service.CreateAccessRequest(ctx, key, grantingBinding)
	if err != nil {
		return nil, h.loggedError(huma.Error500InternalServerError(fmt.Sprintf("error creating access request for role %s", grantingBinding.Spec.RoleTemplateRef.Name), err))
	}

	return &CreateAccessRequestResponse{Body: toAccessRequestResponseBody(ar)}, nil

}

func (h *APIHandler) loggedError(err huma.StatusError) huma.StatusError {
	h.logger.Error(err, "backend error")
	return err
}

// toAccessRequestResponseBody will convert the given ar into an AccessRequestResponseBody.
func toAccessRequestResponseBody(ar *api.AccessRequest) AccessRequestResponseBody {
	expiresAt := ""
	if ar.Status.ExpiresAt != nil {
		expiresAt = ar.Status.ExpiresAt.Format(time.RFC3339)
	}
	requestedAt := ""
	if len(ar.Status.History) > 0 {
		for _, h := range ar.Status.History {
			if h.RequestState == api.InitiatedStatus {
				requestedAt = ar.Status.History[0].TransitionTime.Format(time.RFC3339)
				break
			}
		}
	}
	// This is to keep backwards compatibility as old AccessRequests wont have the InitiatedStatus
	// in the history
	if requestedAt == "" && ar.Status.RequestState != api.InvalidStatus && ar.IsInitialized() {
		requestedAt = ar.GetCreationTimestamp().Format(time.RFC3339)
	}

	message := ""
	if len(ar.Status.History) > 0 && ar.Status.History[len(ar.Status.History)-1].Details != nil {
		message = *ar.Status.History[len(ar.Status.History)-1].Details
	}

	permission := ar.Spec.Role.TemplateRef.Name
	if ar.Spec.Role.FriendlyName != nil {
		permission = *ar.Spec.Role.FriendlyName
	}

	return AccessRequestResponseBody{
		Name:        ar.GetName(),
		Namespace:   ar.GetNamespace(),
		Username:    ar.Spec.Subject.Username,
		Permission:  permission,
		RequestedAt: requestedAt,
		Role:        ar.Spec.Role.TemplateRef.Name,
		Status:      strings.ToUpper(string(ar.Status.RequestState)),
		ExpiresAt:   expiresAt,
		Message:     message,
	}
}

func toListAccessRequestResponseBody(accessRequests []*api.AccessRequest) ListAccessRequestResponseBody {
	items := []AccessRequestResponseBody{}
	for _, ar := range accessRequests {
		items = append(items, toAccessRequestResponseBody(ar))
	}
	return ListAccessRequestResponseBody{Items: items}
}

// listAccessRequestOperation defines the list access requests operation.
func listAccessRequestOperation() huma.Operation {
	return huma.Operation{
		OperationID: "list-accessrequest",
		Method:      http.MethodGet,
		Path:        "/accessrequests",
		Summary:     "List AccessRequests",
		Description: "Will retrieve an ordered list of access requests for the given context",
	}
}

// listAllowedRolesOperation defines the operation to list the user's available
// roles.
func listAllowedRolesOperation() huma.Operation {
	return huma.Operation{
		OperationID: "list-allowedroles",
		Method:      http.MethodGet,
		Path:        "/roles",
		Summary:     "List allowed roles for the user",
		Description: "Will retrieve an ordered list of allowed roles that the user can be elevated to",
	}
}

// createAccessRequestOperation defines the create access request operation.
func createAccessRequestOperation() huma.Operation {
	return huma.Operation{
		OperationID: "create-accessrequest",
		Method:      http.MethodPost,
		Path:        "/accessrequests",
		Summary:     "Create AccessRequest",
		Description: "Will create an access request for the given role and context",
	}
}

// RegisterRoutes will register all routes provided by the access request REST API
// in the given api.
func RegisterRoutes(api huma.API, h *APIHandler) {
	huma.Register(api, listAccessRequestOperation(), h.listAccessRequestHandler)
	huma.Register(api, listAllowedRolesOperation(), h.listAllowedRolesHandler)
	huma.Register(api, createAccessRequestOperation(), h.createAccessRequestHandler)
}
