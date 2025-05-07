package agent

import (
	"context"
	"time"
)

// KubeConfig represents the response for a kubeconfig request
type KubeConfig struct {
	Config   string // The raw kubeconfig content
	Endpoint string // The API server endpoint
}

// KamajiClient defines the interface for interacting with Kamaji API
type KamajiClient interface {
	// CreateTenantControlPlane creates a new tenant control plane (Kubernetes cluster)
	CreateTenantControlPlane(ctx context.Context, name string, version string, replicas int) error

	// DeleteTenantControlPlane deletes an existing tenant control plane
	DeleteTenantControlPlane(ctx context.Context, name string) error

	// WaitForTenantControlPlaneReady waits for a tenant control plane to be ready
	WaitForTenantControlPlaneReady(ctx context.Context, name string) error

	// GetTenantKubeConfig gets the kubeconfig for a tenant control plane
	GetTenantKubeConfig(ctx context.Context, name string) (*KubeConfig, error)

	// GetTenantCerts gets the certificates for a tenant control plane
	GetTenantCAHash(ctx context.Context, name string) (string, error)

	// GetTenantClient gets a subcluster client
	GetTenantClient(ctx context.Context, name string) (KamajiTenantClient, error)
}

// JoinTokenResponse represents a token for joining nodes to a cluster
type JoinTokenResponse struct {
	TokenID        string
	TokenSecret    string
	FullToken      string
	ExpirationTime time.Time
}

// KamajiTenantClient defines the interface for interacting with Kamaji API
type KamajiTenantClient interface {

	// CreateJoinToken creates a bootstrap token for nodes to join the cluster
	CreateJoinToken(ctx context.Context, tenantName string, validityHours int) (*JoinTokenResponse, error)

	// DeleteWorkerNode deletes a worker node
	DeleteWorkerNode(ctx context.Context, nodeName string) error
}
