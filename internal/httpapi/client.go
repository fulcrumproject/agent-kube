package httpapi

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// ClientOption is a function type that configures an HTTPClient
type ClientOption func(*Client)

// AuthType defines the type of authorization header format
type AuthType string

const (
	// AuthTypeBearer uses "Bearer token" format
	AuthTypeBearer AuthType = "Bearer"
	// AuthTypePVE uses "PVEAPIToken=token" format for Proxmox
	AuthTypePVE AuthType = "PVEAPIToken"
)

// WithSkipTLSVerify returns an option that configures TLS certificate validation
func WithSkipTLSVerify(skip bool) ClientOption {
	return func(c *Client) {
		c.skipTLSVerify = skip
	}
}

// WithTimeout returns an option that configures the request timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithAuthType returns an option that configures the authorization type
func WithAuthType(authType AuthType) ClientOption {
	return func(c *Client) {
		c.authType = authType
	}
}

// Client is a generic HTTP client for making API requests
type Client struct {
	BaseURL       string
	HTTPClient    *http.Client
	Token         string // Authentication token
	authType      AuthType
	skipTLSVerify bool
	timeout       time.Duration
}

// NewHTTPClient creates a new HTTP client with the specified base URL and token
func NewHTTPClient(baseURL string, token string, options ...ClientOption) *Client {
	// Create client with default values
	client := &Client{
		BaseURL:       baseURL,
		Token:         token,
		authType:      AuthTypeBearer, // Default to Bearer token
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

// formatAuthHeader formats the authorization header according to the client's auth type
func (c *Client) formatAuthHeader() string {
	switch c.authType {
	case AuthTypePVE:
		return fmt.Sprintf("%s=%s", c.authType, c.Token)
	case AuthTypeBearer:
		fallthrough
	default:
		return fmt.Sprintf("%s %s", c.authType, c.Token)
	}
}

// Get performs an HTTP GET request to the specified endpoint
func (c *Client) Get(endpoint string) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.formatAuthHeader())
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}

// Post performs an HTTP POST request to the specified endpoint with the given body
func (c *Client) Post(endpoint string, body []byte) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)

	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.formatAuthHeader())
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}

// PostForm performs an HTTP POST request with form data to the specified endpoint
func (c *Client) PostForm(endpoint string, formData url.Values) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)

	body := strings.NewReader(formData.Encode())
	req, err := http.NewRequest(http.MethodPost, u.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.formatAuthHeader())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return c.HTTPClient.Do(req)
}

// Put performs an HTTP PUT request to the specified endpoint with the given body
func (c *Client) Put(endpoint string, body []byte) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)

	req, err := http.NewRequest(http.MethodPut, u.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.formatAuthHeader())
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}

// Delete performs an HTTP DELETE request to the specified endpoint
func (c *Client) Delete(endpoint string) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)

	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.formatAuthHeader())
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}

// Patch performs an HTTP PATCH request to the specified endpoint with the given body
func (c *Client) Patch(endpoint string, body []byte) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)

	req, err := http.NewRequest(http.MethodPatch, u.String(), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", c.formatAuthHeader())
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}
