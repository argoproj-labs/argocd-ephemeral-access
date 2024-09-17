package backend

import (
	"context"
	"fmt"
	"net/http"

	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	"github.com/danielgtaylor/huma/v2"
)

const (
	// APITitle refers to the API description used in the open-api spec.
	APITitle = "Ephemeral Access API"
	// APIVersion refers to the API version used in the open-api spec.
	APIVersion = "0.0.1"
)

type GetAccessRequestInput struct {
	Name string `path:"name" maxLength:"100000" example:"some-name" doc:"some name documentation"`
}

type GetAccessRequestResponse struct {
	Body GetAccessRequestResponseBody
}

type GetAccessRequestResponseBody struct {
	Message string `json:"message" example:"hello world" doc:"some message documentation"`
}

type APIHandler struct {
	service Service
	logger  log.Logger
}

func NewAPIHandler(s Service, logger log.Logger) *APIHandler {
	return &APIHandler{
		service: s,
		logger:  logger,
	}
}

func (h *APIHandler) getAccessRequestHandler(ctx context.Context, input *GetAccessRequestInput) (*GetAccessRequestResponse, error) {
	ar, err := h.service.GetAccessRequest(ctx, input.Name, "argocd")
	if err != nil {
		h.logger.Error(err, "error getting accessrequest")
		// TODO
	}
	h.logger.Info(fmt.Sprintf("ar: %+v", ar))

	return &GetAccessRequestResponse{
		Body: GetAccessRequestResponseBody{
			Message: fmt.Sprintf("accessrequest: %+v", ar),
		},
	}, nil
}

func getAccessRequestOperation() huma.Operation {
	return huma.Operation{
		OperationID: "get-accessrequest-by-name",
		Method:      http.MethodGet,
		Path:        "/accessrequests/{name}",
		Summary:     "Get accessrequests by name",
		Description: "Will retrieve the accessrequest by name",
	}
}

func RegisterRoutes(api huma.API, h *APIHandler) {
	huma.Register(api, getAccessRequestOperation(), h.getAccessRequestHandler)
}
