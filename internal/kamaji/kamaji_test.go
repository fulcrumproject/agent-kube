package kamaji

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"fulcrumproject.org/kube-agent/internal/config"
	"fulcrumproject.org/kube-agent/internal/helpers"
	"fulcrumproject.org/kube-agent/internal/httpcli"
	"github.com/stretchr/testify/assert"
)

// TestKamajiIntegration tests the integration with a real Kamaji server
// This test requires a valid .env file with Kubernetes credentials
// It will only run if the INTEGRATION_TEST environment variable is set to true
func TestKamajiIntegration(t *testing.T) {
	helpers.SkipIfNotIntegrationTest(t)

	cfg, err := config.Builder().WithEnv("../..").Build()
	assert.NoError(t, err)

	// Kamaji client uses Kubernetes credentials
	httpCli := httpcli.NewHTTPClient(cfg.KubeAPIURL, cfg.KubeAPIToken,
		httpcli.WithSkipTLSVerify(true)) // Skip TLS verification for test environment
	assert.NotNil(t, httpCli)

	cli := NewKamajiClient(cfg.KubeAPIURL, cfg.KubeAPIToken,
		httpcli.WithSkipTLSVerify(true))
	assert.NotNil(t, cli)

	t.Run("Create, Wait, Test Connection, Create Join Token, Delete Tenant Control Plane", func(t *testing.T) {
		// Define test tenant name with timestamp to avoid conflicts
		timestamp := time.Now().Unix()
		tenantName := fmt.Sprintf("integration-test-%d", timestamp)
		kubeVersion := "v1.28.0" // Use an appropriate version
		replicas := 1

		t.Logf("Starting test: Creating tenant control plane %s with version %s and %d replicas",
			tenantName, kubeVersion, replicas)

		// 1. Create the tenant control plane
		tcpResp, err := cli.CreateTenantControlPlane(tenantName, kubeVersion, replicas)
		assert.NoError(t, err, "CreateTenantControlPlane should not return an error")
		assert.NotNil(t, tcpResp, "CreateTenantControlPlane should return a response")
		assert.Equal(t, tenantName, tcpResp.Metadata.Name, "TCP name should match")

		t.Logf("Tenant control plane %s created successfully", tenantName)

		// 2. Wait for the tenant control plane to be ready
		t.Logf("Waiting for tenant control plane %s to be ready", tenantName)
		err = cli.WaitForTenantControlPlaneReady(tenantName, 300) // 5 minutes timeout
		assert.NoError(t, err, "WaitForTenantControlPlaneReady should not return an error")

		t.Logf("Tenant control plane %s is ready", tenantName)

		// 3. Get the tenant kubeconfig
		t.Logf("Getting kubeconfig for tenant control plane %s", tenantName)
		kubeConfigResp, err := cli.GetTenantKubeconfig(tenantName)
		assert.NoError(t, err, "GetTenantKubeconfig should not return an error")
		assert.NotNil(t, kubeConfigResp, "GetTenantKubeconfig should return a response")
		assert.NotEmpty(t, kubeConfigResp.Config, "Kubeconfig content should not be empty")
		assert.NotEmpty(t, kubeConfigResp.Endpoint, "Kubeconfig endpoint should not be empty")
		assert.NotEmpty(t, kubeConfigResp.SecretName, "Kubeconfig secret name should not be empty")

		t.Logf("Got kubeconfig with endpoint: %s", kubeConfigResp.Endpoint)

		// Test a connection using the kubeconfig
		t.Logf("Testing connection to tenant control plane using kubeconfig")

		// Write kubeconfig to a temporary file
		tmpKubeconfig := fmt.Sprintf("/tmp/kubeconfig-%s", tenantName)
		err = writeKubeconfigToFile(kubeConfigResp.Config, tmpKubeconfig)
		assert.NoError(t, err, "Writing kubeconfig to temp file should not return an error")

		// Use kubectl to check the connection
		cmd := exec.Command("kubectl", "--kubeconfig", tmpKubeconfig, "get", "nodes")
		output, err := cmd.CombinedOutput()
		assert.NoError(t, err, "kubectl should not return an error: %s", string(output))
		t.Logf("Connection test successful: %s", string(output))

		// 4. Create a join token
		t.Logf("Creating join token for tenant control plane %s", tenantName)
		tokenResp, err := cli.CreateJoinToken(tenantName, 24) // 24 hours validity
		assert.NoError(t, err, "CreateJoinToken should not return an error")
		assert.NotNil(t, tokenResp, "CreateJoinToken should return a response")
		assert.NotEmpty(t, tokenResp.FullToken, "Join token should not be empty")
		assert.NotEmpty(t, tokenResp.Endpoint, "Join token endpoint should not be empty")
		assert.NotEmpty(t, tokenResp.CAHash, "Join token CA hash should not be empty")

		t.Logf("Created join token: %s.xxx (partially redacted) valid until %v",
			tokenResp.TokenID, tokenResp.ExpirationTime)

		// 5. Delete the tenant control plane
		t.Logf("Deleting tenant control plane %s", tenantName)
		err = cli.DeleteTenantControlPlane(tenantName)
		assert.NoError(t, err, "DeleteTenantControlPlane should not return an error")

		t.Logf("Tenant control plane %s deleted successfully", tenantName)

		// Clean up the temporary kubeconfig file
		err = deleteFile(tmpKubeconfig)
		if err != nil {
			t.Logf("Warning: Failed to delete temporary kubeconfig file: %v", err)
		}
	})
}

// Helper function to write kubeconfig to file
func writeKubeconfigToFile(content, path string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

// Helper function to delete a file
func deleteFile(path string) error {
	return os.Remove(path)
}
