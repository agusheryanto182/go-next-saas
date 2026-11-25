// Package v1 implements the /api/v1 endpoints defined in openapi/openapi.yaml. The generated
// strict server (package gen) binds requests and writes responses; this package maps them to use cases.
package v1

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	middleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/problem"
	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/v1/gen"
)

// BaseURL is where the contract's paths are served (the spec's `servers` entry).
const BaseURL = "/api"

const _uuidPattern = `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`

// Mount registers every contract route on mux. Requests are validated against the spec first,
// so handlers only see well-formed input; failures become 400 problems naming the field.
func Mount(mux *http.ServeMux, log *slog.Logger) error {
	spec, err := gen.GetSpec()
	if err != nil {
		return fmt.Errorf("load embedded OpenAPI spec: %w", err)
	}

	validate := middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		Options: openapi3filter.Options{
			// Sessions are checked by our own middleware, not by the validator.
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			// Formats are checked here too, so a malformed email is reported like any other field.
			SchemaValidationOptions: []openapi3.SchemaValidationOption{
				openapi3.EnableFormatValidation(),
				openapi3.WithStringFormatValidator("email", openapi3.NewRegexpFormatValidator(openapi3.FormatOfStringForEmail)),
				openapi3.WithStringFormatValidator("uuid", openapi3.NewRegexpFormatValidator(_uuidPattern)),
			},
		},
		DoNotValidateServers: true,
		Prefix:               BaseURL,
		ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, _ *http.Request, opts middleware.ErrorHandlerOpts) {
			problem.Write(w, validationProblem(err, opts.StatusCode))
		},
	})

	strict := gen.NewStrictHandlerWithOptions(&Handler{}, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			problem.Write(w, problem.New(http.StatusBadRequest, err.Error()))
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			log.ErrorContext(r.Context(), "handler failed", slog.Any("error", err), slog.String("path", r.URL.Path))
			problem.Write(w, problem.New(http.StatusInternalServerError, "Something went wrong. Please try again."))
		},
	})

	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{
		BaseURL:     BaseURL,
		BaseRouter:  mux,
		Middlewares: []gen.MiddlewareFunc{validate},
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			problem.Write(w, problem.New(http.StatusBadRequest, err.Error()))
		},
	})
	return nil
}

// validationProblem turns a spec validation failure into a problem with the offending field named.
func validationProblem(err error, suggested int) problem.Problem {
	status := suggested
	if status == 0 {
		status = http.StatusBadRequest
	}
	p := problem.New(status, "The request is invalid.")

	var reqErr *openapi3filter.RequestError
	if !errors.As(err, &reqErr) {
		return p
	}
	switch {
	case reqErr.Parameter != nil:
		reason := reqErr.Reason
		if reqErr.Err != nil {
			reason = reqErr.Err.Error()
		}
		p.Errors = map[string]string{reqErr.Parameter.Name: reason}
	case reqErr.RequestBody != nil:
		p.Errors = map[string]string{bodyField(reqErr): bodyReason(reqErr)}
	}
	return p
}

// bodyField names the body property at fault ("password", "user.email"), or "body" for the whole body.
func bodyField(reqErr *openapi3filter.RequestError) string {
	var schemaErr *openapi3.SchemaError
	if errors.As(reqErr.Err, &schemaErr) {
		if path := schemaErr.JSONPointer(); len(path) > 0 {
			return strings.Join(path, ".")
		}
	}
	return "body"
}

// bodyReason is the rule that failed. Never SchemaError.Error(): it quotes the submitted value,
// which may be a password.
func bodyReason(reqErr *openapi3filter.RequestError) string {
	var schemaErr *openapi3.SchemaError
	if errors.As(reqErr.Err, &schemaErr) && schemaErr.Reason != "" {
		return schemaErr.Reason
	}
	return "The request body is invalid."
}
