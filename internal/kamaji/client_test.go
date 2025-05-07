package kamaji

import (
	"context"
	"fmt"
	"testing"
	"time"

	"fulcrumproject.org/kube-agent/internal/config"
	"github.com/stretchr/testify/require"
)

// TestKamajiClientIntegration tests the integration with a real Kamaji server
// This test requires a valid .env file with Kamaji credentials
// It will only run if the INTEGRATION_TEST environment variable is set to true
func TestKamajiClientIntegration(t *testing.T) {
	// Skip if not an integration test
	// helpers.SkipIfNotIntegrationTest(t)

	// Load configuration from .env file
	cfg, err := config.Builder().WithEnv("../..").Build()
	require.NoError(t, err, "Failed to load configuration")
	require.NotNil(t, cfg, "Configuration should not be nil")
	require.NotEmpty(t, cfg.KubeAPIURL, "KubeAPIURL should not be empty")
	require.NotEmpty(t, cfg.KubeAPIToken, "KubeAPIToken should not be empty")

	// Create client
	client, err := NewClient(cfg.KubeAPIURL, cfg.KubeAPIToken)
	require.NoError(t, err, "Failed to create Kamaji client")
	require.NotNil(t, client, "Kamaji client should not be nil")

	// Generate a unique tenant name to avoid conflicts
	timestamp := time.Now().Unix()
	testTenantName := fmt.Sprintf("test-tenant-%d", timestamp)
	testVersion := "v1.30.2"
	testReplicas := 2

	t.Run("Create, Get, Delete TCP", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		t.Logf("Running integration test for tenant: %s", testTenantName)

		// Create the test tenant control plane
		t.Logf("Creating tenant control plane: %s", testTenantName)
		err := client.CreateTenantControlPlane(ctx, testTenantName, testVersion, testReplicas)

		require.NoError(t, err, "CreateTenantControlPlane should not return an error")
		t.Logf("Tenant control plane created successfully")

		// Wait for the tenant control plane to be ready
		t.Logf("Waiting for tenant control plane to be ready: %s", testTenantName)
		err = client.WaitForTenantControlPlaneReady(ctx, testTenantName) // 60 seconds timeout

		require.NoError(t, err, "WaitForTenantControlPlaneReady should not return an error")
		t.Logf("Tenant control plane is ready")

		// Get the tenant kube config
		t.Logf("Getting kube config for tenant: %s", testTenantName)
		kubeconfigResponse, err := client.GetTenantKubeConfig(ctx, testTenantName)

		require.NoError(t, err, "GetTenantKubeconfig should not return an error")
		require.NotNil(t, kubeconfigResponse, "GetTenantKubeconfig should return a response")
		require.NotEmpty(t, kubeconfigResponse.Config, "Kubeconfig should not be empty")
		require.NotEmpty(t, kubeconfigResponse.Endpoint, "Endpoint should not be empty")
		t.Logf("Tenant kubeconfig retrieved successfully")

		// Get the tenant CA hash
		t.Logf("Getting CA hash for tenant: %s", testTenantName)
		caHash, err := client.GetTenantCAHash(ctx, testTenantName)

		require.NoError(t, err, "GetTenantCAHash should not return an error")
		require.NotEmpty(t, caHash, "CA hash should not be empty")
		require.Contains(t, caHash, "sha256:", "CA hash should be in the format 'sha256:[hash]'")
		t.Logf("Tenant CA hash retrieved successfully: %s", caHash)

		// Get the tenant client
		t.Logf("Getting tenant client for: %s", testTenantName)
		tenantClient, err := client.GetTenantClient(ctx, testTenantName)

		require.NoError(t, err, "GetTenantClient should not return an error")
		require.NotNil(t, tenantClient, "Tenant client should not be nil")
		t.Logf("Tenant client retrieved successfully")

		// Apply Calico resources
		t.Logf("Applying Calico resources to tenant: %s", testTenantName)
		err = tenantClient.CreateCalicoResources(ctx)

		require.NoError(t, err, "ApplyCalicoResources should not return an error")
		t.Logf("Calico resources applied successfully")

		// Create a join token
		t.Logf("Creating join token for tenant: %s", testTenantName)
		tokenResponse, err := tenantClient.CreateJoinToken(ctx, testTenantName, 24)

		require.NoError(t, err, "CreateJoinToken should not return an error")
		require.NotNil(t, tokenResponse, "Token response should not be nil")
		require.NotEmpty(t, tokenResponse.FullToken, "Full token should not be empty")
		require.NotEmpty(t, tokenResponse.TokenID, "Token ID should not be empty")
		require.NotEmpty(t, tokenResponse.TokenSecret, "Token secret should not be empty")
		t.Logf("Join token created successfully")

		// Delete the tenant control plane
		t.Logf("Deleting tenant control plane: %s", testTenantName)
		err = client.DeleteTenantControlPlane(ctx, testTenantName)

		require.NoError(t, err, "DeleteTenantControlPlane should not return an error")

		t.Logf("Tenant control plane deleted successfully")

		// Verify the tenant is gone (this should fail with an error)
		_, err = client.GetTenantKubeConfig(ctx, testTenantName)
		require.Error(t, err, "Getting deleted tenant should return an error")
		t.Logf("Confirmed tenant %s no longer exists", testTenantName)
	})
}
