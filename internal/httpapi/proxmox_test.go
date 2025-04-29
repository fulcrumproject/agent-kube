package httpapi

import (
	"fmt"
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
	cfg, err := config.Builder().WithEnv("../..").Build()
	assert.NoError(t, err)

	cli := NewProxmoxClient(
		cfg.ProxmoxAPIURL,
		cfg.ProxmoxAPIToken,
		cfg.ProxmoxHost,
		cfg.ProxmoxStorage,
		WithSkipTLSVerify(cfg.SkipTLSVerify),
	)

	t.Run("Clone, Config, Start, Stop, Delete VM", func(t *testing.T) {
		// Generate a test VM ID
		testVMID := generateTestVMID()

		// Define test VM name
		vmName := fmt.Sprintf("integration-test-vm-%d", testVMID)

		t.Logf("Starting test: Cloning VM template %d to new VM %d with name %s",
			cfg.ProxmoxTemplate, testVMID, vmName)

		// 1. Clone the VM
		cloneResp, err := cli.CloneVM(cfg.ProxmoxTemplate, testVMID, vmName)
		assert.NoError(t, err, "CloneVM should not return an error")
		assert.NotNil(t, cloneResp, "CloneVM should return a response")
		assert.NotEmpty(t, cloneResp.TaskID, "CloneVM should return a task ID")

		t.Logf("Clone task started with task ID: %s", cloneResp.TaskID)

		// Wait for clone operation to complete (can take a while)
		cloneStatus, err := cli.WaitForTask(cloneResp.TaskID, 5*time.Minute)
		assert.NoError(t, err, "WaitForTask for clone should not return an error")
		assert.Equal(t, "OK", cloneStatus.ExitStatus, "Clone task should complete with OK status")
		assert.Equal(t, "stopped", cloneStatus.Status, "Clone task status should be stopped")

		t.Logf("Clone operation completed successfully")

		// 2. Configure the VM
		t.Logf("Configuring VM with 2 cores and 2048MB memory")
		configResp, err := cli.ConfigureVM(testVMID, 2, 2048, "")
		assert.NoError(t, err, "ConfigureVM should not return an error")
		assert.NotNil(t, configResp, "ConfigureVM should return a response")
		assert.NotEmpty(t, configResp.TaskID, "ConfigureVM should return a task ID")

		// Wait for configure operation to complete
		configStatus, err := cli.WaitForTask(configResp.TaskID, 1*time.Minute)
		assert.NoError(t, err, "WaitForTask for config should not return an error")
		assert.Equal(t, "OK", configStatus.ExitStatus, "Config task should complete with OK status")

		t.Logf("Configure operation completed successfully")

		// 3. Start the VM
		t.Logf("Starting VM %d", testVMID)
		startResp, err := cli.StartVM(testVMID)
		assert.NoError(t, err, "StartVM should not return an error")
		assert.NotNil(t, startResp, "StartVM should return a response")
		assert.NotEmpty(t, startResp.TaskID, "StartVM should return a task ID")

		// Wait for start operation to complete
		startStatus, err := cli.WaitForTask(startResp.TaskID, 3*time.Minute)
		assert.NoError(t, err, "WaitForTask for start should not return an error")
		assert.Equal(t, "OK", startStatus.ExitStatus, "Start task should complete with OK status")

		t.Logf("VM started successfully")

		// 4. Stop the VM
		t.Logf("Stopping VM %d", testVMID)
		stopResp, err := cli.StopVM(testVMID)
		assert.NoError(t, err, "StopVM should not return an error")
		assert.NotNil(t, stopResp, "StopVM should return a response")
		assert.NotEmpty(t, stopResp.TaskID, "StopVM should return a task ID")

		// Wait for stop operation to complete
		stopStatus, err := cli.WaitForTask(stopResp.TaskID, 2*time.Minute)
		assert.NoError(t, err, "WaitForTask for stop should not return an error")
		assert.Equal(t, "OK", stopStatus.ExitStatus, "Stop task should complete with OK status")

		t.Logf("VM stopped successfully")

		// 5. Delete the VM
		t.Logf("Deleting VM %d", testVMID)
		deleteResp, err := cli.DeleteVM(testVMID)
		assert.NoError(t, err, "DeleteVM should not return an error")
		assert.NotNil(t, deleteResp, "DeleteVM should return a response")
		assert.NotEmpty(t, deleteResp.TaskID, "DeleteVM should return a task ID")

		// Wait for delete operation to complete
		deleteStatus, err := cli.WaitForTask(deleteResp.TaskID, 2*time.Minute)
		assert.NoError(t, err, "WaitForTask for delete should not return an error")
		assert.Equal(t, "OK", deleteStatus.ExitStatus, "Delete task should complete with OK status")

		t.Logf("VM deleted successfully")
	})

	t.Run("Clone Non-Existent Template", func(t *testing.T) {
		// Generate a test VM ID
		testVMID := generateTestVMID()

		// Define test VM name
		vmName := fmt.Sprintf("nonexistent-test-vm-%d", testVMID)

		// Use a very high template ID that is unlikely to exist
		nonExistentTemplateID := 999999

		t.Logf("Testing clone of non-existent template: %d to new VM %d with name %s",
			nonExistentTemplateID, testVMID, vmName)

		// Attempt to clone the VM from a non-existent template
		cloneResp, err := cli.CloneVM(nonExistentTemplateID, testVMID, vmName)

		// Should return an error
		assert.Error(t, err, "CloneVM with non-existent template should return an error")
		assert.Nil(t, cloneResp, "CloneVM with non-existent template should return nil response")

		t.Logf("Correctly failed to clone non-existent template with error: %v", err)
	})
}
