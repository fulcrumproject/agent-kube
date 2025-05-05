package kamaji

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"fulcrumproject.org/kube-agent/internal/agent"
	"fulcrumproject.org/kube-agent/internal/httpcli"
)

// HTTPKamajiClient implements the agent.KamajiClient interface
type HTTPKamajiClient struct {
	httpClient        *httpcli.Client
	kubeURL           string
	adminToken        string
	tenantCertCache   map[string]*agent.TenantCerts
	tenantHttpClients map[string]*http.Client
}

// NewKamajiClient creates a new Kamaji API client
func NewKamajiClient(kubeURL string, adminToken string, options ...httpcli.ClientOption) *HTTPKamajiClient {
	return &HTTPKamajiClient{
		httpClient:        httpcli.NewHTTPClient(kubeURL, adminToken, options...),
		kubeURL:           kubeURL,
		adminToken:        adminToken,
		tenantCertCache:   make(map[string]*agent.TenantCerts),
		tenantHttpClients: make(map[string]*http.Client),
	}
}

// CreateTenantControlPlane creates a new tenant control plane (Kubernetes cluster)
func (c *HTTPKamajiClient) CreateTenantControlPlane(name string, version string, replicas int) (*agent.TCPResponse, error) {
	log.Printf("Creating tenant control plane %s with version %s and %d replicas", name, version, replicas)

	// Generate the TCP payload using the template
	params := TCPTemplateParams{
		Name:     name,
		Version:  version,
		Replicas: replicas,
	}
	tcpPayload, err := generateTCPPayload(params)
	if err != nil {
		return nil, fmt.Errorf("failed to generate TCP payload: %w", err)
	}

	// Convert the payload to JSON
	payloadBytes, err := json.Marshal(tcpPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal TCP payload: %w", err)
	}

	// Send the request
	endpoint := "/apis/kamaji.clastix.io/v1alpha1/namespaces/default/tenantcontrolplanes"
	resp, err := c.httpClient.Post(endpoint, payloadBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant control plane: %w", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create tenant control plane, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the response
	var tcpResponse agent.TCPResponse
	if err := json.NewDecoder(resp.Body).Decode(&tcpResponse); err != nil {
		return nil, fmt.Errorf("failed to decode TCP response: %w", err)
	}

	log.Printf("Successfully created tenant control plane %s", name)
	return &tcpResponse, nil
}

// DeleteTenantControlPlane deletes an existing tenant control plane
func (c *HTTPKamajiClient) DeleteTenantControlPlane(name string) error {
	log.Printf("Deleting tenant control plane %s", name)

	// Send the request
	endpoint := fmt.Sprintf("/apis/kamaji.clastix.io/v1alpha1/namespaces/default/tenantcontrolplanes/%s", name)
	resp, err := c.httpClient.Delete(endpoint)
	if err != nil {
		return fmt.Errorf("failed to delete tenant control plane: %w", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete tenant control plane, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Clean up cached certs and clients
	delete(c.tenantCertCache, name)
	delete(c.tenantHttpClients, name)

	log.Printf("Successfully deleted tenant control plane %s", name)
	return nil
}

// GetTenantControlPlane gets information about a specific tenant control plane
func (c *HTTPKamajiClient) GetTenantControlPlane(name string) (*agent.TCPResponse, error) {
	log.Printf("Getting information for tenant control plane %s", name)

	// Send the request
	endpoint := fmt.Sprintf("/apis/kamaji.clastix.io/v1alpha1/namespaces/default/tenantcontrolplanes/%s", name)
	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant control plane: %w", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get tenant control plane, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the response
	var tcpResponse agent.TCPResponse
	if err := json.NewDecoder(resp.Body).Decode(&tcpResponse); err != nil {
		return nil, fmt.Errorf("failed to decode TCP response: %w", err)
	}

	return &tcpResponse, nil
}

// GetTenantControlPlaneStatus gets the status of a tenant control plane
func (c *HTTPKamajiClient) GetTenantControlPlaneStatus(name string) (string, error) {
	tcp, err := c.GetTenantControlPlane(name)
	if err != nil {
		return "", err
	}

	return tcp.Status.KubernetesResources.Version.Status, nil
}

// WaitForTenantControlPlaneReady waits for a tenant control plane to be ready
func (c *HTTPKamajiClient) WaitForTenantControlPlaneReady(name string, timeoutSec int) error {
	log.Printf("Waiting for tenant control plane %s to be ready (timeout: %d seconds)", name, timeoutSec)

	// Calculate the deadline
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)

	// Check the status periodically
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ticker.C:
			status, err := c.GetTenantControlPlaneStatus(name)
			if err != nil {
				log.Printf("Error getting TCP status: %v", err)
				continue
			}

			log.Printf("Tenant control plane %s status: %s", name, status)

			if status == "Ready" {
				log.Printf("Tenant control plane %s is ready", name)
				return nil
			}
		}
	}

	return fmt.Errorf("timed out waiting for tenant control plane %s to be ready", name)
}

// GetTenantKubeconfig gets the kubeconfig for a tenant control plane
func (c *HTTPKamajiClient) GetTenantKubeconfig(name string) (*agent.KubeconfigResponse, error) {
	log.Printf("Getting kubeconfig for tenant control plane %s", name)

	// First get the TCP to find the kubeconfig secret name
	tcp, err := c.GetTenantControlPlane(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant control plane: %w", err)
	}

	secretName := tcp.Status.Kubeconfig.Admin.SecretName
	if secretName == "" {
		return nil, fmt.Errorf("kubeconfig secret name not found for tenant control plane %s", name)
	}

	// Send the request to get the secret
	endpoint := fmt.Sprintf("/api/v1/namespaces/default/secrets/%s", secretName)
	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig secret: %w", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get kubeconfig secret, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the response
	var secretResponse struct {
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&secretResponse); err != nil {
		return nil, fmt.Errorf("failed to decode secret response: %w", err)
	}

	// Extract and decode the kubeconfig
	encodedConfig, ok := secretResponse.Data["admin.conf"]
	if !ok {
		return nil, fmt.Errorf("admin.conf key not found in kubeconfig secret")
	}

	configBytes, err := base64.StdEncoding.DecodeString(encodedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode kubeconfig: %w", err)
	}

	// Save the certificates for later use
	if err := c.cacheTenantCerts(name, string(configBytes)); err != nil {
		log.Printf("Warning: failed to cache tenant certificates: %v", err)
	}

	return &agent.KubeconfigResponse{
		Config:     string(configBytes),
		Endpoint:   tcp.Status.ControlPlaneEndpoint,
		SecretName: secretName,
	}, nil
}

// cacheTenantCerts extracts and caches certificates from the kubeconfig
func (c *HTTPKamajiClient) cacheTenantCerts(tenantName string, kubeconfigContent string) error {
	// This is a simplified implementation. In a real-world scenario, you'd use
	// the k8s.io/client-go/tools/clientcmd package to parse the kubeconfig properly.

	// Extract client certificate
	clientCertStart := strings.Index(kubeconfigContent, "client-certificate-data: ")
	if clientCertStart == -1 {
		return fmt.Errorf("client certificate not found in kubeconfig")
	}
	clientCertStart += len("client-certificate-data: ")
	clientCertEnd := strings.Index(kubeconfigContent[clientCertStart:], "\n")
	if clientCertEnd == -1 {
		return fmt.Errorf("malformed client certificate in kubeconfig")
	}
	encodedClientCert := strings.TrimSpace(kubeconfigContent[clientCertStart : clientCertStart+clientCertEnd])
	clientCert, err := base64.StdEncoding.DecodeString(encodedClientCert)
	if err != nil {
		return fmt.Errorf("failed to decode client certificate: %w", err)
	}

	// Extract client key
	clientKeyStart := strings.Index(kubeconfigContent, "client-key-data: ")
	if clientKeyStart == -1 {
		return fmt.Errorf("client key not found in kubeconfig")
	}
	clientKeyStart += len("client-key-data: ")
	clientKeyEnd := strings.Index(kubeconfigContent[clientKeyStart:], "\n")
	if clientKeyEnd == -1 {
		return fmt.Errorf("malformed client key in kubeconfig")
	}
	encodedClientKey := strings.TrimSpace(kubeconfigContent[clientKeyStart : clientKeyStart+clientKeyEnd])
	clientKey, err := base64.StdEncoding.DecodeString(encodedClientKey)
	if err != nil {
		return fmt.Errorf("failed to decode client key: %w", err)
	}

	// Extract CA certificate
	caStart := strings.Index(kubeconfigContent, "certificate-authority-data: ")
	if caStart == -1 {
		return fmt.Errorf("CA certificate not found in kubeconfig")
	}
	caStart += len("certificate-authority-data: ")
	caEnd := strings.Index(kubeconfigContent[caStart:], "\n")
	if caEnd == -1 {
		return fmt.Errorf("malformed CA certificate in kubeconfig")
	}
	encodedCA := strings.TrimSpace(kubeconfigContent[caStart : caStart+caEnd])
	caCert, err := base64.StdEncoding.DecodeString(encodedCA)
	if err != nil {
		return fmt.Errorf("failed to decode CA certificate: %w", err)
	}

	// Calculate CA certificate hash (in a real implementation, this would compute the proper hash)
	caHash := "sha256:placeholder_hash_would_be_computed_correctly_in_real_implementation"

	// Cache the certificates
	c.tenantCertCache[tenantName] = &agent.TenantCerts{
		ClientCert: clientCert,
		ClientKey:  clientKey,
		CACert:     caCert,
		CAHash:     caHash,
	}

	// Create a tenant-specific HTTP client
	cert, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		return fmt.Errorf("failed to create X509 key pair: %w", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	c.tenantHttpClients[tenantName] = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return nil
}

// getTenantHttpClient returns an HTTP client configured with the tenant's certificates
func (c *HTTPKamajiClient) getTenantHttpClient(tenantName string) (*http.Client, error) {
	client, ok := c.tenantHttpClients[tenantName]
	if !ok {
		return nil, fmt.Errorf("no HTTP client found for tenant %s", tenantName)
	}
	return client, nil
}

// CreateJoinToken creates a bootstrap token for nodes to join the cluster
func (c *HTTPKamajiClient) CreateJoinToken(tenantName string, validityHours int) (*agent.JoinTokenResponse, error) {
	log.Printf("Creating join token for tenant %s (valid for %d hours)", tenantName, validityHours)

	// First, we need the TCP information to get the endpoint
	tcp, err := c.GetTenantControlPlane(tenantName)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant control plane: %w", err)
	}

	// Ensure we have cached certificates
	if _, ok := c.tenantCertCache[tenantName]; !ok {
		// If not, fetch the kubeconfig to cache them
		_, err := c.GetTenantKubeconfig(tenantName)
		if err != nil {
			return nil, fmt.Errorf("failed to get tenant kubeconfig: %w", err)
		}
	}

	// Get the tenant-specific HTTP client
	tenantClient, err := c.getTenantHttpClient(tenantName)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant HTTP client: %w", err)
	}

	// Generate token ID and secret
	tokenID := fmt.Sprintf("%06x", time.Now().UnixNano())[0:6]
	tokenSecret := fmt.Sprintf("%016x", time.Now().UnixNano())

	// Calculate expiration time
	expirationTime := time.Now().Add(time.Duration(validityHours) * time.Hour)
	expirationStr := expirationTime.UTC().Format(time.RFC3339)

	// Generate the token secret payload using template
	params := JoinTokenTemplateParams{
		TokenID:        tokenID,
		TokenSecret:    tokenSecret,
		ExpirationTime: expirationStr,
	}
	payloadBytes, err := generateJoinTokenPayload(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token payload: %w", err)
	}

	// Construct the full URL
	tokenURL := fmt.Sprintf("https://%s/api/v1/namespaces/kube-system/secrets", tcp.Status.ControlPlaneEndpoint)

	// Create the request
	req, err := http.NewRequest("POST", tokenURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := tenantClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create join token: %w", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create join token, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Construct the response
	fullToken := fmt.Sprintf("%s.%s", tokenID, tokenSecret)
	caHash := c.tenantCertCache[tenantName].CAHash

	return &agent.JoinTokenResponse{
		TokenID:        tokenID,
		TokenSecret:    tokenSecret,
		FullToken:      fullToken,
		CAHash:         caHash,
		Endpoint:       tcp.Status.ControlPlaneEndpoint,
		ExpirationTime: expirationTime,
	}, nil
}
