package webapi

import (
	"bytes"
	"crypto/tls"
	"net/http"
	"net/url"
	"path"
	"time"
)

// HTTPClientOption is a function type that configures an HTTPClient
type HTTPClientOption func(*HTTPClient)

// WithSkipTLSVerify returns an option that configures TLS certificate validation
func WithSkipTLSVerify(skip bool) HTTPClientOption {
	return func(c *HTTPClient) {
		c.skipTLSVerify = skip
	}
}

// WithTimeout returns an option that configures the request timeout
func WithTimeout(timeout time.Duration) HTTPClientOption {
	return func(c *HTTPClient) {
		c.timeout = timeout
	}
}

// HTTPClient is a generic HTTP client for making API requests
type HTTPClient struct {
	BaseURL       string
	HTTPClient    *http.Client
	Token         string // Authentication token
	skipTLSVerify bool
	timeout       time.Duration
}

// NewHTTPClient creates a new HTTP client with the specified base URL and token
func NewHTTPClient(baseURL string, token string, options ...HTTPClientOption) *HTTPClient {
	// Create client with default values
	client := &HTTPClient{
		BaseURL:       baseURL,
		Token:         token,
		skipTLSVerify: false,
		timeout:       30 * time.Second,
	}

	// Apply user-provided options
	for _, option := range options {
		option(client)
	}

	// Create transport with TLS config
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: client.skipTLSVerify,
		},
	}

	// Set up the HTTP client
	client.HTTPClient = &http.Client{
		Timeout:   client.timeout,
		Transport: transport,
	}

	return client
}

// Get performs an HTTP GET request to the specified endpoint
func (c *HTTPClient) Get(endpoint string) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}

// Post performs an HTTP POST request to the specified endpoint with the given body
func (c *HTTPClient) Post(endpoint string, body []byte) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)

	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}

// Put performs an HTTP PUT request to the specified endpoint with the given body
func (c *HTTPClient) Put(endpoint string, body []byte) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}

// Delete performs an HTTP DELETE request to the specified endpoint
func (c *HTTPClient) Delete(endpoint string) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)

	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}

// Patch performs an HTTP PATCH request to the specified endpoint with the given body
func (c *HTTPClient) Patch(endpoint string, body []byte) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)

	req, err := http.NewRequest(http.MethodPatch, u.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}
