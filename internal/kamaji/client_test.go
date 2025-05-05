package kamaji

import (
	"fmt"
	"testing"
	"time"

	"fulcrumproject.org/kube-agent/internal/config"
	"github.com/stretchr/testify/assert"
)

// TestKamajiClientIntegration tests the integration with a real Kamaji server
// This test requires a valid .env file with Kamaji credentials
// It will only run if the INTEGRATION_TEST environment variable is set to true
func TestKamajiClientIntegration(t *testing.T) {
	// Skip if not an integration test
	// helpers.SkipIfNotIntegrationTest(t)

	// Load configuration from .env file
	cfg, err := config.Builder().WithEnv("../..").Build()
	assert.NoError(t, err, "Failed to load configuration")
	assert.NotNil(t, cfg, "Configuration should not be nil")
	assert.NotEmpty(t, cfg.KubeAPIURL, "KubeAPIURL should not be empty")
	assert.NotEmpty(t, cfg.KubeAPIToken, "KubeAPIToken should not be empty")

	// Create client
	client, err := NewClient(cfg.KubeAPIURL, cfg.KubeAPIToken)
	assert.NoError(t, err, "Failed to create Kamaji client")
	assert.NotNil(t, client, "Kamaji client should not be nil")

	// Generate a unique tenant name to avoid conflicts
	timestamp := time.Now().Unix()
	testTenantName := fmt.Sprintf("test-tenant-%d", timestamp)
	testVersion := "v1.30.2"
	testReplicas := 2

	t.Run("Create, Get, Delete TCP", func(t *testing.T) {
		// Create the test tenant control plane
		t.Logf("Creating tenant control plane: %s", testTenantName)
		tcpResponse, err := client.CreateTenantControlPlane(testTenantName, testVersion, testReplicas)

		// Assertions
		assert.NoError(t, err, "CreateTenantControlPlane should not return an error")
		assert.NotNil(t, tcpResponse, "CreateTenantControlPlane should return a response")
		assert.Equal(t, testTenantName, tcpResponse.Metadata.Name, "Tenant name should match")
		assert.Equal(t, testVersion, tcpResponse.Spec.Kubernetes.Version, "Kubernetes version should match")
		assert.Equal(t, testReplicas, tcpResponse.Spec.ControlPlane.Deployment.Replicas, "Replicas should match")

		t.Logf("Tenant control plane created successfully")

		// Get the tenant control plane
		t.Logf("Getting tenant control plane: %s", testTenantName)
		tcpResponse, err = client.GetTenantControlPlane(testTenantName)

		// Assertions
		assert.NoError(t, err, "GetTenantControlPlane should not return an error")
		assert.NotNil(t, tcpResponse, "GetTenantControlPlane should return a response")
		assert.Equal(t, testTenantName, tcpResponse.Metadata.Name, "Tenant name should match")
		assert.Equal(t, testVersion, tcpResponse.Spec.Kubernetes.Version, "Kubernetes version should match")

		t.Logf("Tenant control plane retrieved successfully")

		// Delete the tenant control plane
		t.Logf("Deleting tenant control plane: %s", testTenantName)
		err = client.DeleteTenantControlPlane(testTenantName)

		// Assertions
		assert.NoError(t, err, "DeleteTenantControlPlane should not return an error")

		t.Logf("Tenant control plane deleted successfully")

		// Verify the tenant is gone (this should fail with an error)
		_, err = client.GetTenantControlPlane(testTenantName)
		assert.Error(t, err, "Getting deleted tenant should return an error")
		t.Logf("Confirmed tenant %s no longer exists", testTenantName)
	})
}
