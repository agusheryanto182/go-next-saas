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

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/v1/gen"
)

// contractRouter finds the spec operation for a request, so responses can be checked against the contract.
func contractRouter(t *testing.T) routers.Router {
	t.Helper()
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	spec.Servers = nil // match paths as written in the spec, without the /api prefix
	r, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatalf("contract router: %v", err)
	}
	return r
}

// assertMatchesContract fails the test if the response status, content type or body is not what the spec allows.
func assertMatchesContract(t *testing.T, router routers.Router, req *http.Request, rec *httptest.ResponseRecorder) {
	t.Helper()
	if err := contractError(t, router, req, rec); err != nil {
		t.Errorf("response does not match the contract: %v\nbody: %s", err, rec.Body.String())
	}
}

func contractError(t *testing.T, router routers.Router, req *http.Request, rec *httptest.ResponseRecorder) error {
	t.Helper()
	specReq := req.Clone(req.Context())
	specReq.URL.Path = strings.TrimPrefix(req.URL.Path, "/api")
	route, params, err := router.FindRoute(specReq)
	if err != nil {
		t.Fatalf("no spec operation for %s %s: %v", req.Method, req.URL.Path, err)
	}
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
			rec := httptest.NewRecorder()
			write(rec)
			if err := contractError(t, contract, req, rec); err == nil {
				t.Error("violation not detected")
			}
		})
	}
}

// OpenAPI 3.1 nullability (`type: [string, "null"]`) must be understood by the validator.
func TestContractNullableField(t *testing.T) {
	contract := contractRouter(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me", nil)
	user := `{"id":"0193b5a4-6c2e-7d4a-9f1e-2b3c4d5e6f70","email":"a@example.com","fullName":"A","createdAt":"2026-11-23T10:00:00Z","phone":%s}`

	for _, phone := range []string{`null`, `"+62 812 3456 7890"`} {
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		rec.WriteHeader(http.StatusOK)
		_, _ = rec.WriteString(strings.Replace(user, "%s", phone, 1))
		if err := contractError(t, contract, req, rec); err != nil {
			t.Errorf("phone %s rejected: %v", phone, err)
		}
	}

	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.WriteHeader(http.StatusOK)
	_, _ = rec.WriteString(strings.Replace(user, "%s", `42`, 1))
	if err := contractError(t, contract, req, rec); err == nil {
		t.Error("phone 42 accepted; only a string or null is allowed")
	}
}

func TestContractRoutes(t *testing.T) {
	h := newRouter(t, fakeDB{})
	contract := contractRouter(t)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantField  string // field named in the problem's errors map
	}{
		{"me is not implemented yet", "/api/v1/me", http.StatusNotImplemented, ""},
		{"members is not implemented yet", "/api/v1/orgs/acme-corp/members?role=admin&page=1&size=50", http.StatusNotImplemented, ""},
		{"size above the maximum", "/api/v1/orgs/acme-corp/members?size=500", http.StatusBadRequest, "size"},
		{"negative page", "/api/v1/orgs/acme-corp/members?page=-1", http.StatusBadRequest, "page"},
		{"unknown role", "/api/v1/orgs/acme-corp/members?role=boss", http.StatusBadRequest, "role"},
		{"slug breaks the pattern", "/api/v1/orgs/Acme_Corp/members", http.StatusBadRequest, "slug"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			p := decodeProblemKeepBody(t, rec)
			if rec.Code != tt.wantStatus || p.Status != tt.wantStatus {
				t.Fatalf("status = %d, problem = %+v", rec.Code, p)
			}
			if tt.wantField != "" && p.Errors[tt.wantField] == "" {
				t.Errorf("errors = %v, want an entry for %q", p.Errors, tt.wantField)
			}
			assertMatchesContract(t, contract, req, rec)
		})
	}
}

func TestContractRouteRejectsWrongMethod(t *testing.T) {
	rec := serve(t, newRouter(t, fakeDB{}), http.MethodPost, "/api/v1/me")

	if p := decodeProblem(t, rec); rec.Code != http.StatusMethodNotAllowed || p.Status != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, problem = %+v", rec.Code, p)
	}
}
