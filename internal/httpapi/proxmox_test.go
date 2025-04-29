package httpapi

import (
	"testing"
	"time"

	"fulcrumproject.org/kube-agent/internal/config"
	"github.com/stretchr/testify/assert"
)

// generateTestVMID creates a test VM ID that's unlikely to conflict with existing VMs
func generateTestVMID() int {
	timestamp := time.Now().Unix()
	return 900_000 + int(timestamp%10) // VM ID in range 9000-9999
}

// TestCloneVMIntegration tests the integration with a real Proxmox server
// This test requires a valid .env file with Proxmox credentials
// It will only run if the INTEGRATION_TEST environment variable is set to true
func TestVMIntegration(t *testing.T) {
	cfg, err := config.Builder().WithEnv().Build()
	assert.NoError(t, err)

	cli := NewProxmoxClient(
		cfg.ProxmoxAPIURL,
		cfg.ProxmoxAPIToken,
		cfg.ProxmoxHost,
		cfg.ProxmoxStorage,
		WithSkipTLSVerify(cfg.SkipTLSVerify),
	)

	t.Run("Clone, Start, Stop, Delete VM", func(t *testing.T) {
		// Generate a test VM ID
		testVMID := generateTestVMID()

		// Define test VM name
		vmName := "integration-test-vm"

		t.Logf("Starting test: Cloning VM template %d to new VM %d with name %s",
			cfg.ProxmoxTemplate, testVMID, vmName)

		// Test the CloneVM method
		cloneResponse, err := cli.CloneVM(cfg.ProxmoxTemplate, testVMID, vmName)

		// Check for errors
		if err != nil {
			t.Fatalf("CloneVM failed: %v", err)
		}

		// Verify the response
		if cloneResponse == nil {
			t.Fatal("CloneVM returned nil response")
		}
		if cloneResponse.TaskID == "" {
			t.Error("CloneVM returned empty task ID")
		}

		t.Logf("Clone task started with task ID: %s", cloneResponse.TaskID)
	})

}
