package kamaji

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"

	"fulcrumproject.org/kube-agent/internal/agent"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// KamajiNamespace is the namespace where Kamaji resources are created
	KamajiNamespace = "default"

	// TCPGroup is the API group for TenantControlPlane resources
	TCPGroup = "kamaji.clastix.io"

	// TCPVersion is the API version for TenantControlPlane resources
	TCPVersion = "v1alpha1"

	// TCPResource is the resource name for TenantControlPlane
	TCPResource = "tenantcontrolplanes"

	// DefaultTimeout is the default timeout for waiting operations
	DefaultTimeout = 5 * time.Minute

	// PollInterval is the interval between status checks
	PollInterval = 5 * time.Second
)

//go:embed calico.yaml
var calicoYamlContent string

var tcpGVR = schema.GroupVersionResource{
	Group:    TCPGroup,
	Version:  TCPVersion,
	Resource: TCPResource,
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
		CoreDNS      map[string]any `json:"coreDNS"`
		KubeProxy    map[string]any `json:"kubeProxy"`
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

// Client implements the KamajiClient interface using k8s.io client libraries
type Client struct {
	dynamicClient dynamic.Interface
	clientset     kubernetes.Interface
	config        *rest.Config
}

// NewClient creates a new KamajiClient using the provided API URL and token
func NewClient(apiURL, token string) (agent.KamajiClient, error) {
	config := &rest.Config{
		Host:        apiURL,
		BearerToken: token,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true, // You might want to make this configurable
		},
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		dynamicClient: dynamicClient,
		clientset:     clientset,
		config:        config,
	}, nil
}

// CreateTenantControlPlane creates a new tenant control plane (Kubernetes cluster)
func (c *Client) CreateTenantControlPlane(ctx context.Context, name string, version string, replicas int) error {
	// Define the TenantControlPlane resource
	tcp := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": fmt.Sprintf("%s/%s", TCPGroup, TCPVersion),
			"kind":       "TenantControlPlane",
			"metadata": map[string]any{
				"name": name,
				"labels": map[string]any{
					"created-by":        "fulcrum-kube-agent",
					"tenant.clastix.io": name,
				},
			},
			"spec": map[string]any{
				"controlPlane": map[string]any{
					"deployment": map[string]any{
						"replicas": replicas,
					},
					"service": map[string]any{
						"serviceType": "LoadBalancer",
					},
				},
				"kubernetes": map[string]any{
					"version": version,
					"kubelet": map[string]any{
						"cgroupfs": "systemd",
					},
				},
				"networkProfile": map[string]any{
					"port": 6443,
				},
				"addons": map[string]any{
					"coreDNS":   map[string]any{},
					"kubeProxy": map[string]any{},
					"konnectivity": map[string]any{
						"server": map[string]any{
							"port": 8132,
						},
					},
				},
			},
		},
	}

	// Create the resource
	_, err := c.dynamicClient.Resource(tcpGVR).Namespace(KamajiNamespace).Create(
		context.Background(),
		tcp,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to create tenant control plane: %w", err)
	}

	return nil
}

// DeleteTenantControlPlane deletes an existing tenant control plane
func (c *Client) DeleteTenantControlPlane(ctx context.Context, name string) error {
	err := c.dynamicClient.Resource(tcpGVR).Namespace(KamajiNamespace).Delete(
		context.Background(),
		name,
		metav1.DeleteOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to delete tenant control plane: %w", err)
	}
	return nil
}

// WaitForTenantControlPlaneReady waits for a tenant control plane to be ready
func (c *Client) WaitForTenantControlPlaneReady(ctx context.Context, name string) error {
	// Poll the status of the TenantControlPlane resource
	err := wait.PollUntilContextTimeout(ctx, PollInterval, DefaultTimeout, true, func(ctx context.Context) (bool, error) {
		tcp, err := c.getTenantControlPlane(ctx, name)
		if err != nil {
			return false, fmt.Errorf("failed to get tenant control plane: %w", err)
		}

		// Check if the control plane is ready
		if tcp.Status.KubernetesResources.Version.Status == "Ready" {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return fmt.Errorf("failed to wait for tenant control plane to be ready: %w", err)
	}
	return nil
}

// GetTenantKubeconfig gets the kubeconfig for a tenant control plane
func (c *Client) GetTenantKubeConfig(ctx context.Context, name string) (*agent.KubeConfig, error) {
	// First get the TCP to find the secret name
	tcp, err := c.getTenantControlPlane(ctx, name)
	if err != nil {
		return nil, err
	}

	secretName := tcp.Status.Kubeconfig.Admin.SecretName
	if secretName == "" {
		return nil, fmt.Errorf("kubeconfig secret name not found for tenant %s", name)
	}

	// Get the secret containing the kubeconfig
	secret, err := c.clientset.CoreV1().Secrets(KamajiNamespace).Get(
		context.Background(),
		secretName,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig secret: %w", err)
	}

	// Extract kubeconfig from the secret
	kubeconfigBytes, found := secret.Data["admin.conf"]
	if !found {
		return nil, fmt.Errorf("admin.conf key not found in secret %s", secretName)
	}

	return &agent.KubeConfig{
		Config:   string(kubeconfigBytes),
		Endpoint: tcp.Status.ControlPlaneEndpoint,
	}, nil
}

func (c *Client) GetTenantCAHash(ctx context.Context, name string) (string, error) {
	// Get the kubeconfig for the tenant
	kubeConfig, err := c.GetTenantKubeConfig(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to get tenant kubeconfig: %w", err)
	}

	// Parse the kubeconfig to access its contents
	config, err := clientcmd.Load([]byte(kubeConfig.Config))
	if err != nil {
		return "", fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	// Get the CA data from the kubeconfig
	// We use the current-context to find the correct cluster and its certificate data
	currentContext := config.CurrentContext
	if currentContext == "" {
		return "", fmt.Errorf("no current-context found in kubeconfig")
	}

	context, exists := config.Contexts[currentContext]
	if !exists {
		return "", fmt.Errorf("context %s not found in kubeconfig", currentContext)
	}

	clusterName := context.Cluster
	cluster, exists := config.Clusters[clusterName]
	if !exists {
		return "", fmt.Errorf("cluster %s not found in kubeconfig", clusterName)
	}

	// Check if we have CA data
	if len(cluster.CertificateAuthorityData) == 0 {
		return "", fmt.Errorf("no certificate authority data found in kubeconfig for cluster %s", clusterName)
	}

	// Parse the certificate
	block, _ := pem.Decode(cluster.CertificateAuthorityData)
	if block == nil || block.Type != "CERTIFICATE" {
		return "", fmt.Errorf("failed to parse certificate PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse X509 certificate: %w", err)
	}

	// Marshal the public key to DER format
	pubKeyDER, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key to DER format: %w", err)
	}

	// Calculate SHA256 hash of the DER-encoded public key
	hash := sha256.Sum256(pubKeyDER)

	// Encode the hash with proper prefix
	encHash := hex.EncodeToString(hash[:])

	// Return the hash as a hex string
	return fmt.Sprintf("sha256:%s", encHash), nil
}

func (c *Client) getTenantControlPlane(ctx context.Context, name string) (*TCPResponse, error) {
	u, err := c.dynamicClient.Resource(tcpGVR).Namespace(KamajiNamespace).Get(
		context.Background(),
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant control plane: %w", err)
	}

	// Convert the unstructured object to JSON
	jsonData, err := json.Marshal(u.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal unstructured to JSON: %w", err)
	}

	// Unmarshal into the TCPResponse struct
	var response TCPResponse
	if err := json.Unmarshal(jsonData, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to TCP response: %w", err)
	}

	return &response, nil
}

// GetTenantClient gets a subcluster client for the given tenant
func (c *Client) GetTenantClient(ctx context.Context, name string) (agent.KamajiTenantClient, error) {
	// Get the kubeconfig to access the tenant cluster
	kubeconfigResp, err := c.GetTenantKubeConfig(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant kubeconfig: %w", err)
	}

	// Create a client to the tenant cluster using the kubeconfig
	tenantConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfigResp.Config))
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant config from kubeconfig: %w", err)
	}

	tc, err := NewTenantClient(name, tenantConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant client: %w", err)
	}

	return tc, nil
}

// TenantClient implements the KubeClient interface for a specific tenant
type TenantClient struct {
	tenantName    string
	tenantConfig  *rest.Config
	tenantClient  kubernetes.Interface
	dynamicClient *dynamic.DynamicClient
	restMapper    *restmapper.DeferredDiscoveryRESTMapper
}

func NewTenantClient(tenantName string, tenantConfig *rest.Config) (*TenantClient, error) {
	clientset, err := kubernetes.NewForConfig(tenantConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant clientset: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(tenantConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating dynamic client: %w", err)
	}

	// Create a discovery client and a RESTMapper
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(tenantConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating discovery client: %w", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	return &TenantClient{
		tenantName:    tenantName,
		tenantConfig:  tenantConfig,
		tenantClient:  clientset,
		dynamicClient: dynamicClient,
		restMapper:    mapper,
	}, nil
}

// CreateJoinToken creates a bootstrap token for nodes to join the cluster
func (t *TenantClient) CreateJoinToken(ctx context.Context, tenantName string, validityHours int) (*agent.JoinTokenResponse, error) {
	// Generate token ID and secret
	tokenID := generateRandomString(6)
	tokenSecret := generateRandomString(16)
	fullToken := fmt.Sprintf("%s.%s", tokenID, tokenSecret)

	// Calculate expiration time
	if validityHours <= 0 {
		validityHours = 24 // Default to 24 hours
	}
	expirationTime := time.Now().Add(time.Duration(validityHours) * time.Hour)

	// Create the bootstrap token secret
	_, err := createBootstrapTokenSecret(t.tenantClient, tokenID, tokenSecret, expirationTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create bootstrap token: %w", err)
	}

	return &agent.JoinTokenResponse{
		TokenID:        tokenID,
		TokenSecret:    tokenSecret,
		FullToken:      fullToken,
		ExpirationTime: expirationTime,
	}, nil
}

// generateRandomString creates a random string of specified length containing lowercase letters and numbers
func generateRandomString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)

	max := big.NewInt(int64(len(chars)))
	for i := 0; i < length; i++ {
		index, err := rand.Int(rand.Reader, max)
		if err != nil {
			// In production code, we should handle this error more gracefully
			// For now, if there's a problem with random generation, we panic
			panic(err)
		}
		result[i] = chars[index.Int64()]
	}

	return string(result)
}

func createBootstrapTokenSecret(clientset kubernetes.Interface, tokenID, tokenSecret string, expiration time.Time) (*corev1.Secret, error) {
	// Create bootstrap token according to Kubernetes standards
	// See https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("bootstrap-token-%s", tokenID),
			Namespace: "kube-system",
		},
		Type: corev1.SecretTypeBootstrapToken,
		Data: map[string][]byte{
			"token-id":                       []byte(tokenID),
			"token-secret":                   []byte(tokenSecret),
			"expiration":                     []byte(expiration.Format(time.RFC3339)),
			"usage-bootstrap-authentication": []byte("true"),
			"usage-bootstrap-signing":        []byte("true"),
			"auth-extra-groups":              []byte("system:bootstrappers:kubeadm:default-node-token"),
		},
	}

	return clientset.CoreV1().Secrets("kube-system").Create(context.Background(), secret, metav1.CreateOptions{})
}

// DeleteWorkerNode deletes a worker node from the tenant cluster
func (t *TenantClient) DeleteWorkerNode(ctx context.Context, nodeName string) error {
	// Delete the node from the Kubernetes cluster
	err := t.tenantClient.CoreV1().Nodes().Delete(
		context.Background(),
		nodeName,
		metav1.DeleteOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to delete worker node %s: %w", nodeName, err)
	}

	return nil
}

// GetNodeStatus retrieves the status of a node in the tenant cluster
func (t *TenantClient) GetNodeStatus(ctx context.Context, nodeName string) (*agent.NodeStatus, error) {
	// Get the node from the Kubernetes API
	node, err := t.tenantClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	// Check if the node is ready
	isReady := false
	for _, condition := range node.Status.Conditions {
		if condition.Type == "Ready" {
			isReady = condition.Status == "True"
			break
		}
	}

	// Create a map of node addresses
	addresses := make(map[string]string)
	for _, addr := range node.Status.Addresses {
		addresses[string(addr.Type)] = addr.Address
	}

	// Return the node status
	return &agent.NodeStatus{
		Name:           node.Name,
		Ready:          isReady,
		KubeletVersion: node.Status.NodeInfo.KubeletVersion,
		Addresses:      addresses,
		CreatedAt:      node.CreationTimestamp.Time,
	}, nil
}

func (t *TenantClient) CreateCalicoResources(ctx context.Context) error {
	return t.createResources(ctx, calicoYamlContent)
}

func (t *TenantClient) createResources(ctx context.Context, yaml string) error {
	docs := strings.Split(yaml, "---")

	for _, doc := range docs {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		err := t.createResource(ctx, doc)
		if err != nil {
			return fmt.Errorf("error creating resource: %w", err)
		}
	}

	return nil
}

func (t *TenantClient) createResource(ctx context.Context, doc string) error {
	var obj map[string]any
	decoder := yaml.NewYAMLToJSONDecoder(strings.NewReader(doc))
	if err := decoder.Decode(&obj); err != nil {
		return err
	}
	if len(obj) == 0 {
		return nil
	}

	u := &unstructured.Unstructured{Object: obj}

	// Get the REST mapping from the RESTMapper
	mapping, err := t.restMapper.RESTMapping(u.GroupVersionKind().GroupKind(), u.GroupVersionKind().Version)
	if err != nil {
		return fmt.Errorf("failed to get REST mapping for %s: %w", u.GroupVersionKind(), err)
	}

	resourceClient := t.dynamicClient.Resource(mapping.Resource).Namespace(u.GetNamespace())
	_, err = resourceClient.Create(ctx, u, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("error creating %s '%s' in %s: %w", u.GetKind(), u.GetName(), u.GetNamespace(), err)
	}

	return nil
}
