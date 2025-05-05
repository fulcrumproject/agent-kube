package agent

import (
	"time"
)

// KamajiClient defines the interface for interacting with Kamaji API
type KamajiClient interface {
	// CreateTenantControlPlane creates a new tenant control plane (Kubernetes cluster)
	CreateTenantControlPlane(name string, version string, replicas int) (*TCPResponse, error)

	// DeleteTenantControlPlane deletes an existing tenant control plane
	DeleteTenantControlPlane(name string) error

	// GetTenantControlPlane gets information about a specific tenant control plane
	GetTenantControlPlane(name string) (*TCPResponse, error)

	// GetTenantControlPlaneStatus gets the status of a tenant control plane
	GetTenantControlPlaneStatus(name string) (string, error)

	// WaitForTenantControlPlaneReady waits for a tenant control plane to be ready
	WaitForTenantControlPlaneReady(name string, timeoutSec int) error

	// GetTenantKubeconfig gets the kubeconfig for a tenant control plane
	GetTenantKubeconfig(name string) (*KubeconfigResponse, error)

	// GetTenantClient gets a subcluster client
	GetTenantClient(name string) (KamajiTenantClient, error)
}

// KamajiTenantClient defines the interface for interacting with Kamaji API
type KamajiTenantClient interface {

	// CreateJoinToken creates a bootstrap token for nodes to join the cluster
	CreateJoinToken(tenantName string, validityHours int) (*JoinTokenResponse, error)

	// DeleteWorkerNode deletes a worker node
	DeleteWorkerNode(nodeName string) error
}

// TCPResponse represents the response from the Kamaji API for TCP operations
type TCPResponse struct {
	ApiVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec   TCPSpec   `json:"spec"`
	Status TCPStatus `json:"status,omitempty"`
}

// TCPSpec represents the specification of a TenantControlPlane
type TCPSpec struct {
	ControlPlane struct {
		Deployment struct {
			Replicas int `json:"replicas"`
		} `json:"deployment"`
		Service struct {
			ServiceType string `json:"serviceType"`
		} `json:"service"`
	} `json:"controlPlane"`
	Kubernetes struct {
		Version string `json:"version"`
		Kubelet struct {
			Cgroupfs string `json:"cgroupfs"`
		} `json:"kubelet"`
	} `json:"kubernetes"`
	NetworkProfile struct {
		Port int `json:"port"`
	} `json:"networkProfile"`
	Addons struct {
		CoreDNS      map[string]interface{} `json:"coreDNS"`
		KubeProxy    map[string]interface{} `json:"kubeProxy"`
		Konnectivity struct {
			Server struct {
				Port int `json:"port"`
			} `json:"server"`
		} `json:"konnectivity"`
	} `json:"addons"`
}

// TCPStatus represents the status of a TenantControlPlane
type TCPStatus struct {
	ControlPlaneEndpoint string `json:"controlPlaneEndpoint"`
	KubernetesResources  struct {
		Version struct {
			Status string `json:"status"`
		} `json:"version"`
	} `json:"kubernetesResources"`
	Kubeconfig struct {
		Admin struct {
			SecretName string `json:"secretName"`
		} `json:"admin"`
	} `json:"kubeconfig"`
}

// KubeconfigResponse represents the response for a kubeconfig request
type KubeconfigResponse struct {
	Config     string // The raw kubeconfig content
	Endpoint   string // The API server endpoint
	SecretName string // The name of the secret containing the kubeconfig
}

// TenantCerts holds the certificates for authenticating with a tenant cluster
type TenantCerts struct {
	ClientCert     []byte
	ClientKey      []byte
	CACert         []byte
	CAHash         string
	KubeconfigPath string
}

// JoinTokenResponse represents a token for joining nodes to a cluster
type JoinTokenResponse struct {
	TokenID        string
	TokenSecret    string
	FullToken      string
	CAHash         string
	Endpoint       string
	ExpirationTime time.Time
}
