package v1

import (
	"context"
	"net/http"

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/v1/gen"
)

// Handler implements gen.StrictServerInterface. Endpoints answer 501 until their milestone implements them.
type Handler struct{}

var _ gen.StrictServerInterface = (*Handler)(nil)

// Login signs in (#38).
func (h *Handler) Login(context.Context, gen.LoginRequestObject) (gen.LoginResponseObject, error) {
	return gen.LogindefaultApplicationProblemPlusJSONResponse{StatusCode: http.StatusNotImplemented, Body: notImplemented()}, nil
}

// Logout signs out (#38).
func (h *Handler) Logout(context.Context, gen.LogoutRequestObject) (gen.LogoutResponseObject, error) {
	return gen.LogoutdefaultApplicationProblemPlusJSONResponse{StatusCode: http.StatusNotImplemented, Body: notImplemented()}, nil
}

// RequestPasswordReset emails a password reset link (#40).
func (h *Handler) RequestPasswordReset(context.Context, gen.RequestPasswordResetRequestObject) (gen.RequestPasswordResetResponseObject, error) {
	return gen.RequestPasswordResetdefaultApplicationProblemPlusJSONResponse{StatusCode: http.StatusNotImplemented, Body: notImplemented()}, nil
}

// InspectToken checks a link token (#40).
func (h *Handler) InspectToken(context.Context, gen.InspectTokenRequestObject) (gen.InspectTokenResponseObject, error) {
	return gen.InspectTokendefaultApplicationProblemPlusJSONResponse{StatusCode: http.StatusNotImplemented, Body: notImplemented()}, nil
}

// ResetPassword sets a new password with a reset token (#40).
func (h *Handler) ResetPassword(context.Context, gen.ResetPasswordRequestObject) (gen.ResetPasswordResponseObject, error) {
	return gen.ResetPassworddefaultApplicationProblemPlusJSONResponse{StatusCode: http.StatusNotImplemented, Body: notImplemented()}, nil
}

// ListMyOrganizations returns the signed-in user's organizations (#39).
func (h *Handler) ListMyOrganizations(context.Context, gen.ListMyOrganizationsRequestObject) (gen.ListMyOrganizationsResponseObject, error) {
	return gen.ListMyOrganizationsdefaultApplicationProblemPlusJSONResponse{StatusCode: http.StatusNotImplemented, Body: notImplemented()}, nil
}

// GetMe returns the signed-in user (#39).
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
