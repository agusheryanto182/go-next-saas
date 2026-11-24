package v1

import (
	"context"
	"net/http"

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/v1/gen"
)

// Handler implements gen.StrictServerInterface. Endpoints answer 501 until their milestone implements them.
type Handler struct{}

var _ gen.StrictServerInterface = (*Handler)(nil)

// GetMe returns the signed-in user (M1).
func (h *Handler) GetMe(context.Context, gen.GetMeRequestObject) (gen.GetMeResponseObject, error) {
	return gen.GetMedefaultApplicationProblemPlusJSONResponse{StatusCode: http.StatusNotImplemented, Body: notImplemented()}, nil
}

// ListMembers returns one page of an organization's members (M2).
func (h *Handler) ListMembers(context.Context, gen.ListMembersRequestObject) (gen.ListMembersResponseObject, error) {
	return gen.ListMembersdefaultApplicationProblemPlusJSONResponse{StatusCode: http.StatusNotImplemented, Body: notImplemented()}, nil
}

func notImplemented() gen.Problem {
	detail := "This endpoint is not implemented yet."
	return gen.Problem{
		Type:   "about:blank",
		Title:  http.StatusText(http.StatusNotImplemented),
		Status: http.StatusNotImplemented,
		Detail: &detail,
	}
}
