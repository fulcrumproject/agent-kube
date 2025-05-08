package proxmox

import (
	"fmt"
	"testing"
	"time"

	"fulcrumproject.org/kube-agent/internal/config"
	"fulcrumproject.org/kube-agent/internal/helpers"
	"fulcrumproject.org/kube-agent/internal/httpcli"
	"fulcrumproject.org/kube-agent/internal/ssh"
	"github.com/stretchr/testify/require"
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
	// Skip if not an integration test
	helpers.SkipIfNotIntegrationTest(t)

	cfg, err := config.Builder().WithEnv("../..").Build()
	require.NoError(t, err)

	httpCli := httpcli.NewHTTPClient(cfg.ProxmoxAPIURL, cfg.ProxmoxAPIToken,
		httpcli.WithAuthType(httpcli.AuthTypePVE),
		httpcli.WithSkipTLSVerify(true)) // Skip TLS verification for test environment
	require.NotNil(t, httpCli)

	cli := NewProxmoxClient(cfg.ProxmoxHost, cfg.ProxmoxStorage, httpCli)
	require.NotNil(t, cli)

	scpOpts := ssh.Options{
		Host:           cfg.ProxmoxCIHost,
		Username:       cfg.ProxmoxCIUser,
		PrivateKeyPath: cfg.ProxmoxCIPKPath,
		Timeout:        30 * time.Second,
	}

	t.Run("Clone, Config, Start, Stop, Delete VM", func(t *testing.T) {
		// Generate a test VM ID
		testVMID := generateTestVMID()

		// Define test VM name
		vmName := fmt.Sprintf("integration-test-vm-%d", testVMID)

		t.Logf("Starting test: Cloning VM template %d to new VM %d with name %s",
			cfg.ProxmoxTemplate, testVMID, vmName)

		// 1. Clone the VM
		cloneResp, err := cli.CloneVM(cfg.ProxmoxTemplate, testVMID, vmName)
		require.NoError(t, err, "CloneVM should not return an error")
		require.NotNil(t, cloneResp, "CloneVM should return a response")
		require.NotEmpty(t, cloneResp.TaskID, "CloneVM should return a task ID")

		t.Logf("Clone task started with task ID: %s", cloneResp.TaskID)

		// Wait for clone operation to complete (can take a while)
		cloneStatus, err := cli.WaitForTask(cloneResp.TaskID, 5*time.Minute)
		require.NoError(t, err, "WaitForTask for clone should not return an error")
		require.Equal(t, "OK", cloneStatus.ExitStatus, "Clone task should complete with OK status")
		require.Equal(t, "stopped", cloneStatus.Status, "Clone task status should be stopped")

		t.Logf("Clone operation completed successfully")

		// 2. Generate and upload cloud-init configuration
		t.Logf("Generating and uploading cloud-init configuration")
		cloudInitParams := CloudInitParams{
			Hostname:       vmName,
			FQDN:           vmName,
			Username:       "ubuntu",
			Password:       "ubuntu",
			SSHKeys:        []string{"ssh-rsa AAAAB3NzaC1yc2EAAAA... test@example.com"},
			ExpirePassword: false,
			PackageUpgrade: true,
			JoinURL:        "172.30.232.66:6443",
			JoinToken:      "123456.6357ad0f550c8e04",
			CACertHash:     "sha256:1992ff0cf2bc550fd67ad3238e1355a47ce6b2f32a009f433139b5985066db54",
			KubeVersion:    "v1.30.5",
		}

		// Generate cloud-init file
		cloudInitContent, err := GenerateCloudInit(CloudInitTestTempl, cloudInitParams)
		require.NoError(t, err, "GenerateCloudInitFile should not return an error")

		// Upload cloud-init file via SCP
		cloudInitFileName := fmt.Sprintf("kube-agent-ci-%s.yml", vmName)
		cloudInitFilePath := fmt.Sprintf("%s/%s", cfg.ProxmoxCIPath, cloudInitFileName)

		// Upload the cloud-init file to the Proxmox server
		err = ssh.CopyFile(scpOpts, []byte(cloudInitContent), cloudInitFilePath)
		require.NoError(t, err, "Uploading cloud-init file via SCP should not return an error")

		t.Logf("Cloud-init configuration uploaded successfully")

		// 3. Configure the VM with cloud-init
		t.Logf("Configuring VM with 2 cores, 2048MB memory, and cloud-init")
		cloudInitConfig := fmt.Sprintf("user=local:snippets/%s", cloudInitFileName)
		configResp, err := cli.ConfigureVM(testVMID, 2, 2048, cloudInitConfig)
		require.NoError(t, err, "ConfigureVM should not return an error")
		require.NotNil(t, configResp, "ConfigureVM should return a response")
		require.NotEmpty(t, configResp.TaskID, "ConfigureVM should return a task ID")

		// Wait for configure operation to complete
		configStatus, err := cli.WaitForTask(configResp.TaskID, 1*time.Minute)
		require.NoError(t, err, "WaitForTask for config should not return an error")
		require.Equal(t, "OK", configStatus.ExitStatus, "Config task should complete with OK status")

		t.Logf("Configure operation completed successfully")

		// 4. Start the VM
		t.Logf("Starting VM %d", testVMID)
		startResp, err := cli.StartVM(testVMID)
		require.NoError(t, err, "StartVM should not return an error")
		require.NotNil(t, startResp, "StartVM should return a response")
		require.NotEmpty(t, startResp.TaskID, "StartVM should return a task ID")

		// Wait for start operation to complete
		startStatus, err := cli.WaitForTask(startResp.TaskID, 3*time.Minute)
		require.NoError(t, err, "WaitForTask for start should not return an error")
		require.Equal(t, "OK", startStatus.ExitStatus, "Start task should complete with OK status")

		t.Logf("VM started successfully")

		// 5. Stop the VM
		t.Logf("Stopping VM %d", testVMID)
		stopResp, err := cli.StopVM(testVMID)
		require.NoError(t, err, "StopVM should not return an error")
		require.NotNil(t, stopResp, "StopVM should return a response")
		require.NotEmpty(t, stopResp.TaskID, "StopVM should return a task ID")

		// Wait for stop operation to complete
		stopStatus, err := cli.WaitForTask(stopResp.TaskID, 2*time.Minute)
		require.NoError(t, err, "WaitForTask for stop should not return an error")
		require.Equal(t, "OK", stopStatus.ExitStatus, "Stop task should complete with OK status")

		t.Logf("VM stopped successfully")

		// 6. Delete the VM
		t.Logf("Deleting VM %d", testVMID)
		deleteResp, err := cli.DeleteVM(testVMID)
		require.NoError(t, err, "DeleteVM should not return an error")
		require.NotNil(t, deleteResp, "DeleteVM should return a response")
		require.NotEmpty(t, deleteResp.TaskID, "DeleteVM should return a task ID")

		// Wait for delete operation to complete
		deleteStatus, err := cli.WaitForTask(deleteResp.TaskID, 2*time.Minute)
		require.NoError(t, err, "WaitForTask for delete should not return an error")
		require.Equal(t, "OK", deleteStatus.ExitStatus, "Delete task should complete with OK status")

		t.Logf("VM deleted successfully")

		// Cleanup CI file
		err = ssh.DeleteFile(scpOpts, cloudInitFilePath)
		if err != nil {
			t.Fatalf("DeleteFile failed: %v", err)
		}
		t.Log("Successfully deleted ci file")
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
		require.Error(t, err, "CloneVM with non-existent template should return an error")
		require.Nil(t, cloneResp, "CloneVM with non-existent template should return nil response")

		t.Logf("Correctly failed to clone non-existent template with error: %v", err)
	})
}
