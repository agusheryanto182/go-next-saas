package restapi

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/v1/gen"
)

// contractRouter finds the spec operation for a request, so responses can be checked against the contract.
func contractRouter(t *testing.T) routers.Router {
	t.Helper()
	spec, err := gen.GetSpec()
	require.NoError(t, err, "load spec")
	spec.Servers = nil // match paths as written in the spec, without the /api prefix
	r, err := gorillamux.NewRouter(spec)
	require.NoError(t, err, "contract router")
	return r
}

// assertMatchesContract fails the test if the response status, content type or body is not what the spec allows.
func assertMatchesContract(t *testing.T, router routers.Router, req *http.Request, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.NoError(t, contractError(t, router, req, rec), "response does not match the contract\nbody: %s", rec.Body.String())
}

func contractError(t *testing.T, router routers.Router, req *http.Request, rec *httptest.ResponseRecorder) error {
	t.Helper()
	specReq := req.Clone(req.Context())
	specReq.URL.Path = strings.TrimPrefix(req.URL.Path, "/api")
	route, params, err := router.FindRoute(specReq)
	require.NoError(t, err, "no spec operation for %s %s", req.Method, req.URL.Path)
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request: specReq, PathParams: params, Route: route,
			Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		},
		Status: rec.Code,
		Header: rec.Header(),
		Body:   io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
	}
	return openapi3filter.ValidateResponse(req.Context(), in)
}

// The checker itself must catch violations, or every contract assertion above would pass vacuously.
func TestContractCheckerCatchesViolations(t *testing.T) {
	t.Parallel()
	contract := contractRouter(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me", nil)

	tests := map[string]func(rec *httptest.ResponseRecorder){
		"user without required fields": func(rec *httptest.ResponseRecorder) {
			rec.Header().Set("Content-Type", "application/json")
			rec.WriteHeader(http.StatusOK)
			_, _ = rec.WriteString(`{"id":"not-a-uuid"}`)
		},
		"error as plain text": func(rec *httptest.ResponseRecorder) {
			rec.Header().Set("Content-Type", "text/plain")
			rec.WriteHeader(http.StatusUnauthorized)
			_, _ = rec.WriteString("unauthorized")
		},
	}
	for name, write := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			write(rec)
			assert.Error(t, contractError(t, contract, req, rec), "violation not detected")
		})
	}
}

// OpenAPI 3.1 nullability (`type: [string, "null"]`) must be understood by the validator.
func TestContractNullableField(t *testing.T) {
	t.Parallel()
	contract := contractRouter(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me", nil)
	user := `{"id":"0193b5a4-6c2e-7d4a-9f1e-2b3c4d5e6f70","email":"a@example.com","fullName":"A","createdAt":"2026-11-23T10:00:00Z","phone":%s}`

	for _, phone := range []string{`null`, `"+62 812 3456 7890"`} {
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		rec.WriteHeader(http.StatusOK)
		_, _ = rec.WriteString(strings.Replace(user, "%s", phone, 1))
		assert.NoError(t, contractError(t, contract, req, rec), "phone %s rejected", phone)
	}

	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.WriteHeader(http.StatusOK)
	_, _ = rec.WriteString(strings.Replace(user, "%s", `42`, 1))
	assert.Error(t, contractError(t, contract, req, rec), "phone 42 accepted; only a string or null is allowed")
}

func TestContractRoutes(t *testing.T) {
	t.Parallel()
	h := newRouter(t, fakeDB{})
	contract := contractRouter(t)

	const token = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJ0123456" // 43 base64url characters
	long := strings.Repeat("a", 129)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantField  string // field named in the problem's errors map
	}{
		{"me is not implemented yet", http.MethodGet, "/api/v1/me", "", http.StatusNotImplemented, ""},
		{"my organizations is not implemented yet", http.MethodGet, "/api/v1/me/organizations", "", http.StatusNotImplemented, ""},
		{"members is not implemented yet", http.MethodGet, "/api/v1/orgs/acme-corp/members?role=admin&page=1&size=50", "", http.StatusNotImplemented, ""},
		{"size above the maximum", http.MethodGet, "/api/v1/orgs/acme-corp/members?size=500", "", http.StatusBadRequest, "size"},
		{"negative page", http.MethodGet, "/api/v1/orgs/acme-corp/members?page=-1", "", http.StatusBadRequest, "page"},
		{"unknown role", http.MethodGet, "/api/v1/orgs/acme-corp/members?role=boss", "", http.StatusBadRequest, "role"},
		{"slug breaks the pattern", http.MethodGet, "/api/v1/orgs/Acme_Corp/members", "", http.StatusBadRequest, "slug"},

		{"login is not implemented yet", http.MethodPost, "/api/v1/auth/login", `{"email":"a@example.com","password":"x"}`, http.StatusNotImplemented, ""},
		{"login without password", http.MethodPost, "/api/v1/auth/login", `{"email":"a@example.com"}`, http.StatusBadRequest, "password"},
		{"login with a password above 128", http.MethodPost, "/api/v1/auth/login", `{"email":"a@example.com","password":"` + long + `"}`, http.StatusBadRequest, "password"},
		{"login with a malformed email", http.MethodPost, "/api/v1/auth/login", `{"email":"not-an-email","password":"x"}`, http.StatusBadRequest, "email"},
		{"login with an unknown field", http.MethodPost, "/api/v1/auth/login", `{"email":"a@example.com","password":"x","admin":true}`, http.StatusBadRequest, "body"},
		{"logout is not implemented yet", http.MethodPost, "/api/v1/auth/logout", "", http.StatusNotImplemented, ""},
		{"forgot password is not implemented yet", http.MethodPost, "/api/v1/auth/forgot-password", `{"email":"a@example.com"}`, http.StatusNotImplemented, ""},
		{"forgot password without email", http.MethodPost, "/api/v1/auth/forgot-password", `{}`, http.StatusBadRequest, "email"},
		{"inspect is not implemented yet", http.MethodPost, "/api/v1/auth/tokens/inspect", `{"token":"` + token + `"}`, http.StatusNotImplemented, ""},
		{"inspect with a malformed token", http.MethodPost, "/api/v1/auth/tokens/inspect", `{"token":"short"}`, http.StatusBadRequest, "token"},
		{"reset is not implemented yet", http.MethodPost, "/api/v1/auth/reset-password", `{"token":"` + token + `","password":"long enough"}`, http.StatusNotImplemented, ""},
		{"reset with a password of 7", http.MethodPost, "/api/v1/auth/reset-password", `{"token":"` + token + `","password":"1234567"}`, http.StatusBadRequest, "password"},
		{"reset with a password above 128", http.MethodPost, "/api/v1/auth/reset-password", `{"token":"` + token + `","password":"` + long + `"}`, http.StatusBadRequest, "password"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, body)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			p := decodeProblemKeepBody(t, rec)
			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, tt.wantStatus, p.Status)
			if tt.wantField != "" {
				assert.NotEmpty(t, p.Errors[tt.wantField], "errors = %v, want an entry for %q", p.Errors, tt.wantField)
			}
			assertMatchesContract(t, contract, req, rec)
		})
	}
}

func TestContractRouteRejectsWrongMethod(t *testing.T) {
	t.Parallel()
	rec := serve(t, newRouter(t, fakeDB{}), http.MethodPost, "/api/v1/me")

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, http.StatusMethodNotAllowed, decodeProblem(t, rec).Status)
}

// Validation problems name the field and the rule, never the submitted value: it may be a password.
func TestValidationProblemDoesNotEchoTheValue(t *testing.T) {
	t.Parallel()
	secret := "SuperSecret-" + strings.Repeat("x", 120)
	body := `{"token":"abcdefghijklmnopqrstuvwxyzABCDEFGHIJ0123456","password":"` + secret + `"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/reset-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newRouter(t, fakeDB{}).ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.NotContains(t, rec.Body.String(), "SuperSecret")
}
