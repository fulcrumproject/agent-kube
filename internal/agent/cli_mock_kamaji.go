package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockTenantControlPlane represents a tenant control plane in the in-memory stub
type MockTenantControlPlane struct {
	Name         string
	Version      string
	Replicas     int
	Status       string // "Provisioning", "Ready"
	Endpoint     string
	CAHash       string
	KubeConfig   string
	CreationTime time.Time
	mu           sync.RWMutex
}

// MockKamajiClient implements KamajiClient interface for testing
type MockKamajiClient struct {
	tenantControlPlanes map[string]*MockTenantControlPlane
	mu                  sync.RWMutex
}

// NewMockKamajiClient creates a new in-memory stub Kamaji client
func NewMockKamajiClient() *MockKamajiClient {
	return &MockKamajiClient{
		tenantControlPlanes: make(map[string]*MockTenantControlPlane),
	}
}

// CreateTenantControlPlane creates a new tenant control plane
func (c *MockKamajiClient) CreateTenantControlPlane(ctx context.Context, name string, version string, replicas int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.tenantControlPlanes[name]; exists {
		return fmt.Errorf("tenant control plane %s already exists", name)
	}

	c.tenantControlPlanes[name] = &MockTenantControlPlane{
		Name:         name,
		Version:      version,
		Replicas:     replicas,
		Status:       "Ready", // Set initially as ready for simplicity in tests
		Endpoint:     fmt.Sprintf("https://%s.example.com:6443", name),
		CAHash:       fmt.Sprintf("sha256:test-ca-hash-for-%s", name),
		KubeConfig:   fmt.Sprintf("apiVersion: v1\nkind: Config\nclusters:\n- cluster:\n    server: https://%s.example.com:6443\n  name: %s", name, name),
		CreationTime: time.Now(),
	}

	return nil
}

// DeleteTenantControlPlane deletes an existing tenant control plane
func (c *MockKamajiClient) DeleteTenantControlPlane(ctx context.Context, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.tenantControlPlanes[name]; !exists {
		return fmt.Errorf("tenant control plane %s not found", name)
	}

	delete(c.tenantControlPlanes, name)
	return nil
}

// SetTenantControlPlaneStatus sets the status of a tenant control plane (for test setup)
func (c *MockKamajiClient) SetTenantControlPlaneStatus(name, status string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tcp, exists := c.tenantControlPlanes[name]
	if !exists {
		return fmt.Errorf("tenant control plane %s not found", name)
	}

	tcp.Status = status
	return nil
}

// WaitForTenantControlPlaneReady waits for a tenant control plane to be ready
func (c *MockKamajiClient) WaitForTenantControlPlaneReady(ctx context.Context, name string) error {
	c.mu.RLock()
	tcp, exists := c.tenantControlPlanes[name]
	if !exists {
		c.mu.RUnlock()
		return fmt.Errorf("tenant control plane %s not found", name)
	}

	status := tcp.Status
	c.mu.RUnlock()

	// In the stub implementation, we simply check the current status
	if status != "Ready" {
		return fmt.Errorf("tenant control plane %s is not ready, current status: %s", name, status)
	}

	return nil
}

// GetTenantKubeConfig gets the kubeconfig for a tenant control plane
func (c *MockKamajiClient) GetTenantKubeConfig(ctx context.Context, name string) (*KubeConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tcp, exists := c.tenantControlPlanes[name]
	if !exists {
		return nil, fmt.Errorf("tenant control plane %s not found", name)
	}

	return &KubeConfig{
		Config:   tcp.KubeConfig,
		Endpoint: tcp.Endpoint,
	}, nil
}

// GetTenantCAHash gets the certificate hash for a tenant control plane
func (c *MockKamajiClient) GetTenantCAHash(ctx context.Context, name string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tcp, exists := c.tenantControlPlanes[name]
	if !exists {
		return "", fmt.Errorf("tenant control plane %s not found", name)
	}

	return tcp.CAHash, nil
}

// GetTenantClient gets a client for interacting with a tenant cluster
func (c *MockKamajiClient) GetTenantClient(ctx context.Context, name string) (KamajiTenantClient, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tcp, exists := c.tenantControlPlanes[name]
	if !exists {
		return nil, fmt.Errorf("tenant control plane %s not found", name)
	}

	return NewStubKamajiTenantClient(tcp), nil
}

// StubKamajiTenantClient implements KamajiTenantClient for testing
type StubKamajiTenantClient struct {
	tcp    *MockTenantControlPlane
	tokens map[string]*JoinTokenResponse
}

// NewStubKamajiTenantClient creates a new tenant client for testing
func NewStubKamajiTenantClient(tcp *MockTenantControlPlane) *StubKamajiTenantClient {
	return &StubKamajiTenantClient{
		tcp:    tcp,
		tokens: make(map[string]*JoinTokenResponse),
	}
}

// CreateJoinToken creates a token for nodes to join the cluster
func (t *StubKamajiTenantClient) CreateJoinToken(ctx context.Context, tenantName string, validityHours int) (*JoinTokenResponse, error) {
	if validityHours <= 0 {
		validityHours = 24 // Default to 24 hours
	}

	tokenID := "test-token-id"
	tokenSecret := "test-token-secret"
	fullToken := fmt.Sprintf("%s.%s", tokenID, tokenSecret)
	expirationTime := time.Now().Add(time.Duration(validityHours) * time.Hour)

	token := &JoinTokenResponse{
		TokenID:        tokenID,
		TokenSecret:    tokenSecret,
		FullToken:      fullToken,
		ExpirationTime: expirationTime,
	}

	t.tcp.mu.Lock()
	defer t.tcp.mu.Unlock()
	t.tokens[tokenID] = token

	return token, nil
}

// DeleteWorkerNode deletes a worker node
func (t *StubKamajiTenantClient) DeleteWorkerNode(ctx context.Context, nodeName string) error {
	// In the stub, we just pretend to delete the node
	return nil
}

// GetNodeStatus gets the status of a node
func (t *StubKamajiTenantClient) GetNodeStatus(ctx context.Context, nodeName string) (*KubeNodeStatus, error) {
	return &KubeNodeStatus{
		Name:           nodeName,
		Ready:          true,
		KubeletVersion: "v1.21.0",
		Addresses:      map[string]string{"InternalIP": "1.2.3.4"},
		CreatedAt:      time.Now(),
	}, nil
}

// CreateCalicoResources applies Calico networking resources to the tenant cluster
func (t *StubKamajiTenantClient) CreateCalicoResources(ctx context.Context) error {
	// In the stub, we just pretend to apply the resources
	return nil
}
