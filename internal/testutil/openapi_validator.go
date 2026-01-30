// Package testutil provides testing utilities for integration tests.
package testutil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
)

// OpenAPIValidator validates HTTP requests and responses against an OpenAPI specification.
type OpenAPIValidator struct {
	doc    *openapi3.T
	router routers.Router
}

// NewOpenAPIValidator creates a new OpenAPI validator from a spec file.
// The specPath should be relative to the test working directory or absolute.
func NewOpenAPIValidator(t *testing.T, specPath string) *OpenAPIValidator {
	t.Helper()

	v, err := LoadOpenAPIValidator(specPath)
	if err != nil {
		t.Fatalf("load OpenAPI validator: %v", err)
	}
	return v
}

// LoadOpenAPIValidator loads and validates an OpenAPI spec, returning a validator.
// Use this in TestMain where *testing.T is not available.
func LoadOpenAPIValidator(specPath string) (*OpenAPIValidator, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("load OpenAPI spec from %s: %w", specPath, err)
	}

	// Validate the spec itself
	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validate OpenAPI spec: %w", err)
	}

	router, err := legacy.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("create OpenAPI router: %w", err)
	}

	return &OpenAPIValidator{
		doc:    doc,
		router: router,
	}, nil
}

// shouldSkipValidation returns true for endpoints that don't return JSON
// or shouldn't be validated (health checks, etc).
func (v *OpenAPIValidator) shouldSkipValidation(path string) bool {
	return path == "/healthz" || path == "/readyz"
}

// ValidateRequest validates an HTTP request against the OpenAPI spec.
// Returns nil if valid, or an error describing the validation failure.
func (v *OpenAPIValidator) ValidateRequest(t *testing.T, req *http.Request) {
	t.Helper()

	if v.shouldSkipValidation(req.URL.Path) {
		return
	}

	route, pathParams, err := v.router.FindRoute(req)
	if err != nil {
		t.Errorf("OpenAPI: no route found for %s %s: %v", req.Method, req.URL.Path, err)
		return
	}

	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			// Don't fail on unknown query parameters
			MultiError: true,
		},
	}

	if err := openapi3filter.ValidateRequest(context.Background(), requestValidationInput); err != nil {
		t.Errorf("OpenAPI request validation failed for %s %s: %v", req.Method, req.URL.Path, err)
	}
}

// ValidateResponse validates an HTTP response against the OpenAPI spec.
// The request is needed to find the corresponding route.
// Note: This consumes and restores the response body.
func (v *OpenAPIValidator) ValidateResponse(t *testing.T, req *http.Request, resp *http.Response) {
	t.Helper()

	if v.shouldSkipValidation(req.URL.Path) {
		return
	}

	// Create a minimal request with just path for route matching.
	// The OpenAPI router expects paths relative to the server base URL.
	routeReq, err := http.NewRequest(req.Method, req.URL.Path, nil)
	if err != nil {
		t.Errorf("create route request: %v", err)
		return
	}

	route, pathParams, err := v.router.FindRoute(routeReq)
	if err != nil {
		t.Errorf("OpenAPI: no route found for %s %s: %v", req.Method, req.URL.Path, err)
		return
	}

	// Read and restore body for validation
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("read response body: %v", err)
		return
	}
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))

	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: pathParams,
		Route:      route,
	}

	responseValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestValidationInput,
		Status:                 resp.StatusCode,
		Header:                 resp.Header,
		Body:                   io.NopCloser(bytes.NewReader(body)),
		Options: &openapi3filter.Options{
			MultiError:            true,
			IncludeResponseStatus: true,
		},
	}

	if err := openapi3filter.ValidateResponse(context.Background(), responseValidationInput); err != nil {
		// Provide helpful error message
		errMsg := err.Error()
		// Truncate very long error messages
		if len(errMsg) > 500 {
			errMsg = errMsg[:500] + "..."
		}
		t.Errorf("OpenAPI response validation failed for %s %s (status %d):\n%s\nResponse body: %s",
			req.Method, req.URL.Path, resp.StatusCode, errMsg, truncateBody(body))
	}
}

// ValidateRequestResponse validates both request and response.
// This is a convenience method that calls both ValidateRequest and ValidateResponse.
func (v *OpenAPIValidator) ValidateRequestResponse(t *testing.T, req *http.Request, resp *http.Response) {
	t.Helper()
	v.ValidateRequest(t, req)
	v.ValidateResponse(t, req, resp)
}

// truncateBody truncates a response body for error reporting.
func truncateBody(body []byte) string {
	s := string(body)
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
