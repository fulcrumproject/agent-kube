package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"fulcrumproject.org/kube-agent/internal/cloudinit"
	"fulcrumproject.org/kube-agent/internal/config"
	"fulcrumproject.org/kube-agent/internal/httpcli"
	"fulcrumproject.org/kube-agent/internal/kamaji"
	"fulcrumproject.org/kube-agent/internal/proxmox"
	"fulcrumproject.org/kube-agent/internal/ssh"
	"fulcrumproject.org/kube-agent/internal/testhelp"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

// TestKamajiProxmoxIntegration tests the full flow of:
// 1. Creating a Kubernetes tenant with Kamaji
// 2. Getting join token and configuration
// 3. Creating a VM in Proxmox configured to join the Kubernetes tenant
func TestKamajiProxmoxIntegration(t *testing.T) {
	// Skip if not an integration test
	testhelp.SkipIfNotIntegrationTest(t)

	// Load configuration from .env file
	cfg, err := config.Builder().WithEnv().Build()
	require.NoError(t, err, "Failed to load configuration")
	require.NotNil(t, cfg, "Configuration should not be nil")

	// Validate Kamaji configuration
	require.NotEmpty(t, cfg.KubeAPIURL, "KubeAPIURL should not be empty")
	require.NotEmpty(t, cfg.KubeAPIToken, "KubeAPIToken should not be empty")

	// Validate Proxmox configuration
	require.NotEmpty(t, cfg.ProxmoxAPIURL, "ProxmoxAPIURL should not be empty")
	require.NotEmpty(t, cfg.ProxmoxAPIToken, "ProxmoxAPIToken should not be empty")
	require.NotEmpty(t, cfg.ProxmoxHost, "ProxmoxHost should not be empty")
	require.NotEmpty(t, cfg.ProxmoxStorage, "ProxmoxStorage should not be empty")
	require.NotEmpty(t, cfg.ProxmoxTemplate, "ProxmoxTemplate should not be empty")
	require.NotEmpty(t, cfg.ProxmoxCIHost, "ProxmoxCIHost should not be empty")
	require.NotEmpty(t, cfg.ProxmoxCIUser, "ProxmoxCIUser should not be empty")
	require.NotEmpty(t, cfg.ProxmoxCIPath, "ProxmoxCIPath should not be empty")
	require.NotEmpty(t, cfg.ProxmoxCIPKPath, "ProxmoxCIPKPath should not be empty")

	// Create Kamaji client
	kamajiClient, err := kamaji.NewClient(cfg.KubeAPIURL, cfg.KubeAPIToken)
	require.NoError(t, err, "Failed to create Kamaji client")
	require.NotNil(t, kamajiClient, "Kamaji client should not be nil")

	// Create Proxmox client
	httpCli := httpcli.NewHTTPClient(
		cfg.ProxmoxAPIURL,
		cfg.ProxmoxAPIToken,
		httpcli.WithAuthType(httpcli.AuthTypePVE),
		httpcli.WithSkipTLSVerify(true), // Skip TLS verification for test
	)
	require.NotNil(t, httpCli)

	proxmoxClient := proxmox.NewProxmoxClient(cfg.ProxmoxHost, cfg.ProxmoxStorage, httpCli)
	require.NotNil(t, proxmoxClient, "Proxmox client should not be nil")

	// SCP configuration for cloud-init
	scpOpts := ssh.Options{
		Host:           cfg.ProxmoxCIHost,
		Username:       cfg.ProxmoxCIUser,
		PrivateKeyPath: cfg.ProxmoxCIPKPath,
		Timeout:        30 * time.Second,
	}

	t.Run("Full Kamaji-Proxmox Integration", func(t *testing.T) {
		// Setup timeouts and context
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Generate a unique tenant name
		timestamp := time.Now().Unix()
		testTenantName := fmt.Sprintf("test-tenant-%d", timestamp)
		testVersion := "v1.30.2"
		testReplicas := 2

		// Generate a test VM ID
		testVMID := testhelp.GenerateTestVMID()
		vmName := fmt.Sprintf("worker-node-%d", testVMID)

		// Create the Kubernetes tenant control plane
		t.Logf("Creating tenant control plane: %s", testTenantName)
		err := kamajiClient.CreateTenantControlPlane(ctx, testTenantName, testVersion, testReplicas)
		require.NoError(t, err, "CreateTenantControlPlane should not return an error")
		t.Logf("Tenant control plane created successfully")

		// Always clean up the tenant at the end
		defer func() {
			t.Logf("Cleanup: Deleting tenant control plane: %s", testTenantName)
			err := kamajiClient.DeleteTenantControlPlane(ctx, testTenantName)
			if err != nil {
				t.Logf("Warning: Failed to delete tenant: %v", err)
			} else {
				t.Logf("Tenant deleted successfully")
			}
		}()

		// Wait for the tenant control plane to be ready
		t.Logf("Waiting for tenant control plane to be ready: %s", testTenantName)
		err = kamajiClient.WaitForTenantControlPlaneReady(ctx, testTenantName)
		require.NoError(t, err, "WaitForTenantControlPlaneReady should not return an error")
		t.Logf("Tenant control plane is ready")

		// Get the tenant kubeconfig and client
		t.Logf("Getting tenant client for: %s", testTenantName)
		tenantClient, err := kamajiClient.GetTenantClient(ctx, testTenantName)
		require.NoError(t, err, "GetTenantClient should not return an error")
		require.NotNil(t, tenantClient, "Tenant client should not be nil")

		// Apply Calico networking resources
		t.Logf("Applying Calico resources to tenant: %s", testTenantName)
		err = tenantClient.CreateCalicoResources(ctx)
		require.NoError(t, err, "CreateCalicoResources should not return an error")
		t.Logf("Calico resources applied successfully")

		// Create a join token for nodes
		t.Logf("Creating join token for tenant: %s", testTenantName)
		tokenResponse, err := tenantClient.CreateJoinToken(ctx, testTenantName, 24)
		require.NoError(t, err, "CreateJoinToken should not return an error")
		require.NotNil(t, tokenResponse, "Token response should not be nil")
		require.NotEmpty(t, tokenResponse.FullToken, "Full token should not be empty")
		t.Logf("Join token created successfully")

		// Get the CA hash
		t.Logf("Getting CA hash for tenant: %s", testTenantName)
		caHash, err := kamajiClient.GetTenantCAHash(ctx, testTenantName)
		require.NoError(t, err, "GetTenantCAHash should not return an error")
		require.NotEmpty(t, caHash, "CA hash should not be empty")
		t.Logf("Tenant CA hash retrieved successfully: %s", caHash)

		// Get the tenant endpoint from kubeconfig
		kubeConfig, err := kamajiClient.GetTenantKubeConfig(ctx, testTenantName)
		require.NoError(t, err, "GetTenantKubeConfig should not return an error")
		require.NotNil(t, kubeConfig, "KubeConfig should not be nil")
		require.NotEmpty(t, kubeConfig.Endpoint, "Endpoint should not be empty")
		t.Logf("Tenant endpoint: %s", kubeConfig.Endpoint)

		// Create a VM in Proxmox that will join the cluster
		t.Logf("Cloning VM template %d to new VM %d with name %s",
			cfg.ProxmoxTemplate, testVMID, vmName)

		// Always clean up the VM at the end
		defer func() {
			t.Logf("Cleanup: Deleting VM %d", testVMID)
			// Stop VM first if needed
			stopResp, err := proxmoxClient.StopVM(testVMID)
			if err != nil {
				t.Logf("Warning: Failed to stop VM: %v", err)
			} else {
				t.Logf("VM stop task started: %s", stopResp.TaskID)
				_, err = proxmoxClient.WaitForTask(stopResp.TaskID, 2*time.Minute)
				if err != nil {
					t.Logf("Warning: Error waiting for VM to stop: %v", err)
				}
			}

			// Now try to delete
			deleteResp, err := proxmoxClient.DeleteVM(testVMID)
			if err != nil {
				t.Logf("Warning: Failed to delete VM: %v", err)
			} else {
				t.Logf("VM delete task started: %s", deleteResp.TaskID)
				_, err = proxmoxClient.WaitForTask(deleteResp.TaskID, 2*time.Minute)
				if err != nil {
					t.Logf("Warning: Error waiting for VM to be deleted: %v", err)
				} else {
					t.Logf("VM deleted successfully")
				}
			}

			// Cleanup cloud-init file if it was created
			cloudInitFileName := fmt.Sprintf("kube-agent-ci-%s.yml", vmName)
			cloudInitFilePath := fmt.Sprintf("%s/%s", cfg.ProxmoxCIPath, cloudInitFileName)
			err = ssh.DeleteFile(scpOpts, cloudInitFilePath)
			if err != nil {
				t.Logf("Warning: Failed to delete cloud-init file: %v", err)
			} else {
				t.Logf("Cloud-init file deleted successfully")
			}

			// Also delete the node from the Kubernetes cluster
			t.Logf("Cleanup: Removing node %s from the Kubernetes cluster", vmName)
			err = tenantClient.DeleteWorkerNode(ctx, vmName)
			if err != nil {
				t.Logf("Warning: Failed to delete node from Kubernetes: %v", err)
			} else {
				t.Logf("Node removed from Kubernetes cluster successfully")
			}
		}()

		// Clone the VM from template
		cloneResp, err := proxmoxClient.CloneVM(cfg.ProxmoxTemplate, testVMID, vmName)
		require.NoError(t, err, "CloneVM should not return an error")
		require.NotNil(t, cloneResp, "CloneVM should return a response")
		require.NotEmpty(t, cloneResp.TaskID, "CloneVM should return a task ID")
		t.Logf("Clone task started with task ID: %s", cloneResp.TaskID)

		// Wait for clone to complete
		cloneStatus, err := proxmoxClient.WaitForTask(cloneResp.TaskID, 5*time.Minute)
		require.NoError(t, err, "WaitForTask for clone should not return an error")
		require.Equal(t, "OK", cloneStatus.ExitStatus, "Clone task should complete with OK status")
		t.Logf("VM cloning completed successfully")

		// Generate and upload cloud-init configuration for joining the Kubernetes cluster
		t.Logf("Generating cloud-init configuration for joining Kubernetes cluster")

		// Parse the endpoint URL to get just the host:port part for joining
		apiServerEndpoint := kubeConfig.Endpoint

		// Generate cloud-init configuration for joining the cluster
		cloudInitParams := cloudinit.CloudInitParams{
			Hostname:       vmName,
			FQDN:           vmName,
			Username:       "ubuntu",
			Password:       "ubuntu",
			SSHKeys:        []string{"ssh-rsa AAAAB3NzaC1yc2EAAAA... test@example.com"}, // Use a real key in prod
			ExpirePassword: false,
			PackageUpgrade: true,
			JoinURL:        apiServerEndpoint,
			JoinToken:      tokenResponse.FullToken,
			CACertHash:     caHash,
			KubeVersion:    testVersion,
		}

		// Generate the cloud-init file content
		cloudInitContent, err := cloudinit.GenerateCloudInit(cloudinit.CloudInitTempl, cloudInitParams)
		require.NoError(t, err, "GenerateCloudInit should not return an error")

		// Upload cloud-init file via SCP
		// Note: We're using SCP for cloud-init file upload as it's more reliable than using
		// the Proxmox API for uploading to the snippets storage
		cloudInitFileName := fmt.Sprintf("kube-agent-ci-%s.yml", vmName)
		cloudInitFilePath := fmt.Sprintf("%s/%s", cfg.ProxmoxCIPath, cloudInitFileName)

		// Upload the cloud-init file to the Proxmox server
		err = ssh.CopyFile(scpOpts, []byte(cloudInitContent), cloudInitFilePath)
		require.NoError(t, err, "Uploading cloud-init file via SCP should not return an error")
		t.Logf("Cloud-init configuration uploaded successfully")

		// Configure the VM with the cloud-init file
		t.Logf("Configuring VM with cloud-init for Kubernetes join")
		cloudInitConfig := fmt.Sprintf("user=local:snippets/%s", cloudInitFileName)
		configResp, err := proxmoxClient.ConfigureVM(testVMID, 2, 2048, cloudInitConfig)
		require.NoError(t, err, "ConfigureVM should not return an error")
		require.NotNil(t, configResp, "ConfigureVM should return a response")

		// Wait for configure to complete
		configStatus, err := proxmoxClient.WaitForTask(configResp.TaskID, 1*time.Minute)
		require.NoError(t, err, "WaitForTask for config should not return an error")
		require.Equal(t, "OK", configStatus.ExitStatus, "Config task should complete with OK status")
		t.Logf("VM configuration completed successfully")

		// Start the VM to join the Kubernetes cluster
		t.Logf("Starting VM to join Kubernetes cluster")
		startResp, err := proxmoxClient.StartVM(testVMID)
		require.NoError(t, err, "StartVM should not return an error")
		require.NotNil(t, startResp, "StartVM should return a response")

		// Wait for VM to start
		startStatus, err := proxmoxClient.WaitForTask(startResp.TaskID, 2*time.Minute)
		require.NoError(t, err, "WaitForTask for start should not return an error")
		require.Equal(t, "OK", startStatus.ExitStatus, "Start task should complete with OK status")
		t.Logf("VM started successfully, joining Kubernetes cluster")

		// Now we'll wait for the node to join the cluster and verify it's properly registered
		t.Logf("Waiting for node to join the Kubernetes cluster...")

		// Poll for node status periodically
		err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 10*time.Minute, true, func(ctx context.Context) (bool, error) {
			nodeStatus, err := tenantClient.GetNodeStatus(ctx, vmName)
			if err != nil {
				t.Logf("Node not yet registered: %v", err)
				return false, nil // Continue polling
			}

			t.Logf("Node found with status - Name: %s, Ready: %t, Version: %s",
				nodeStatus.Name, nodeStatus.Ready, nodeStatus.KubeletVersion)

			// If the node is found but not ready, continue waiting
			if !nodeStatus.Ready {
				t.Logf("Node registered but not yet ready")
				return false, nil
			}

			// Node is registered and ready
			return true, nil
		})

		require.NoError(t, err, "Node should join the cluster and be ready")

		t.Logf("Integration test completed - Kamaji tenant created and Proxmox VM joined as a worker node")
	})
}
