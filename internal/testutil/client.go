// Package testutil provides testing utilities for integration tests.
package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"testing"

	"github.com/bissquit/incident-garden/internal/pkg/httputil"
)

// Client is an HTTP client for testing API endpoints.
type Client struct {
	BaseURL     string
	Token       string // For backward compatibility with Authorization header
	CSRFToken   string // CSRF token for cookie-based auth
	HTTPClient  *http.Client
	Validator   *OpenAPIValidator
	ValidateAPI bool
	t           *testing.T
}

// NewClient creates a new test client without validation.
func NewClient(baseURL string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Jar: jar},
	}
}

// NewClientWithValidation creates a new test client with OpenAPI validation enabled.
// The specPath should be the path to the OpenAPI specification file.
func NewClientWithValidation(t *testing.T, baseURL, specPath string) *Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	return &Client{
		BaseURL:     baseURL,
		HTTPClient:  &http.Client{Jar: jar},
		Validator:   NewOpenAPIValidator(t, specPath),
		ValidateAPI: true,
		t:           t,
	}
}

// NewClientWithValidator creates a new test client with a pre-loaded OpenAPI validator.
// Use this in TestMain where *testing.T is not available during initialization.
func NewClientWithValidator(baseURL string, validator *OpenAPIValidator) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		BaseURL:     baseURL,
		HTTPClient:  &http.Client{Jar: jar},
		Validator:   validator,
		ValidateAPI: true,
	}
}

// SetT sets the testing.T for validation error reporting.
// This should be called at the beginning of each test when using a shared client.
func (c *Client) SetT(t *testing.T) {
	c.t = t
}

// WithoutValidation returns a copy of the client with validation disabled.
// Use this for negative tests where you expect invalid responses.
func (c *Client) WithoutValidation() *Client {
	clone := *c
	clone.ValidateAPI = false
	return &clone
}

// LoginAs authenticates using email/password.
// Cookies (access_token, refresh_token, csrf_token) are automatically stored in the cookie jar.
func (c *Client) LoginAs(t *testing.T, email, password string) {
	t.Helper()
	c.t = t

	resp, err := c.POST("/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	})
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("login failed: status=%d body=%s", resp.StatusCode, body)
	}

	// Extract CSRF token from cookies for subsequent requests
	for _, cookie := range resp.Cookies() {
		if cookie.Name == httputil.CSRFTokenCookie {
			c.CSRFToken = cookie.Value
			break
		}
	}
}

// LoginAsAdmin logs in as admin@example.com.
func (c *Client) LoginAsAdmin(t *testing.T) {
	t.Helper()
	c.LoginAs(t, "admin@example.com", "admin123")
}

// LoginAsOperator logs in as operator@example.com.
func (c *Client) LoginAsOperator(t *testing.T) {
	t.Helper()
	c.LoginAs(t, "operator@example.com", "admin123")
}

// LoginAsUser logs in as user@example.com.
func (c *Client) LoginAsUser(t *testing.T) {
	t.Helper()
	c.LoginAs(t, "user@example.com", "user123")
}

// ClearToken removes the stored token and resets the cookie jar.
func (c *Client) ClearToken() {
	c.Token = ""
	c.CSRFToken = ""
	jar, _ := cookiejar.New(nil)
	c.HTTPClient.Jar = jar
}

// GET performs a GET request.
func (c *Client) GET(path string) (*http.Response, error) {
	return c.do("GET", path, nil)
}

// POST performs a POST request with JSON body.
func (c *Client) POST(path string, body interface{}) (*http.Response, error) {
	return c.do("POST", path, body)
}

// PUT performs a PUT request with JSON body.
func (c *Client) PUT(path string, body interface{}) (*http.Response, error) {
	return c.do("PUT", path, body)
}

// PATCH performs a PATCH request with JSON body.
func (c *Client) PATCH(path string, body interface{}) (*http.Response, error) {
	return c.do("PATCH", path, body)
}

// DELETE performs a DELETE request.
func (c *Client) DELETE(path string) (*http.Response, error) {
	return c.do("DELETE", path, nil)
}

// DELETEWithBody performs a DELETE request with JSON body.
func (c *Client) DELETEWithBody(path string, body interface{}) (*http.Response, error) {
	return c.do("DELETE", path, body)
}

func (c *Client) do(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	var bodyBytes []byte

	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Use Authorization header if Token is set (backward compatibility for API clients)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	// Add CSRF header for state-changing methods when using cookie-based auth
	if c.CSRFToken != "" && isStateChanging(method) {
		req.Header.Set(httputil.CSRFTokenHeader, c.CSRFToken)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Validate response against OpenAPI spec if enabled
	if c.ValidateAPI && c.Validator != nil && c.t != nil {
		// Create a new request for validation (original body was consumed)
		if bodyBytes != nil {
			bodyReader = bytes.NewReader(bodyBytes)
		}
		validationReq, _ := http.NewRequest(method, c.BaseURL+path, bodyReader)
		validationReq.Header = req.Header
		validationReq.URL = req.URL

		c.Validator.ValidateResponse(c.t, validationReq, resp)
	}

	return resp, nil
}

// isStateChanging returns true for HTTP methods that modify state.
func isStateChanging(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// DecodeJSON decodes response body into v.
func DecodeJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// ReadBody reads and returns response body as string.
func ReadBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}
