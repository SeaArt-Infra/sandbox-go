package core

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Transport is the shared HTTP entry point for the SDK.
type Transport struct {
	baseURL    *url.URL
	apiKey     string
	httpClient *http.Client
	userAgent  string
	projectID  string
}

type TransportOption func(*Transport)

// WithHTTPClient replaces the default HTTP client for custom transport, proxy, or timeout control.
func WithHTTPClient(httpClient *http.Client) TransportOption {
	return func(c *Transport) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithTimeout extends client-side request deadlines for long waitReady or polling calls.
func WithTimeout(timeout time.Duration) TransportOption {
	return func(c *Transport) {
		if timeout > 0 {
			c.httpClient.Timeout = timeout
		}
	}
}

// WithProjectID injects X-Project-ID on every control/build-plane request.
func WithProjectID(projectID string) TransportOption {
	return func(c *Transport) {
		c.projectID = strings.TrimSpace(projectID)
	}
}

// NewTransport creates a shared authenticated transport for X-API-Key requests.
func NewTransport(baseURL, apiKey string, opts ...TransportOption) (*Transport, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, ErrBaseURLEmpty
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, ErrAPIKeyEmpty
	}

	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, &url.Error{Op: "parse", URL: baseURL, Err: ErrInvalidBaseURL}
	}

	transport := &Transport{
		baseURL:    parsed,
		apiKey:     strings.TrimSpace(apiKey),
		httpClient: &http.Client{Timeout: 4 * time.Minute},
		userAgent:  UserAgent("seainfra-sandbox-go"),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(transport)
		}
	}
	return transport, nil
}

// BaseURL returns the normalized base URL.
func (c *Transport) BaseURL() string {
	return c.baseURL.String()
}

// NewRequest prepares an authenticated request against the configured API host.
func (c *Transport) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	endpoint, err := c.resolve(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", c.userAgent)
	if c.projectID != "" {
		req.Header.Set("X-Project-ID", c.projectID)
	}
	return req, nil
}

// Do sends a prepared request with the shared HTTP transport.
func (c *Transport) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// DoJSON sends a JSON request, validates the response status, and decodes the body.
func (c *Transport) DoJSON(
	ctx context.Context,
	method, path string,
	headers http.Header,
	query url.Values,
	body any,
	out any,
	expectedStatus ...int,
) (*http.Response, error) {
	resp, err := c.DoRequest(ctx, method, path, headers, query, body, expectedStatus...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return resp, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, err
	}
	return resp, nil
}

// DoRequest sends a request with optional JSON body and query string.
func (c *Transport) DoRequest(
	ctx context.Context,
	method, path string,
	headers http.Header,
	query url.Values,
	body any,
	expectedStatus ...int,
) (*http.Response, error) {
	endpoint := path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	req, err := c.NewRequest(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	if statusAllowed(resp.StatusCode, expectedStatus) {
		return resp, nil
	}

	defer resp.Body.Close()
	return nil, DecodeAPIError(resp)
}

func (c *Transport) resolve(path string) (string, error) {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		normalized = "/"
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}

	ref, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}
	joined, err := url.JoinPath(c.baseURL.String(), strings.TrimPrefix(ref.EscapedPath(), "/"))
	if err != nil {
		return "", err
	}
	resolved, err := url.Parse(joined)
	if err != nil {
		return "", err
	}
	resolved.RawQuery = ref.RawQuery
	resolved.Fragment = ref.Fragment
	return resolved.String(), nil
}

func statusAllowed(statusCode int, expected []int) bool {
	for _, code := range expected {
		if statusCode == code {
			return true
		}
	}
	return false
}
