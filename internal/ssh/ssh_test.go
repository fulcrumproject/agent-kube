package ssh

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"fulcrumproject.org/kube-agent/internal/config"
)

// TestCopyFile tests the CopyFile function with options from .env config
// This test requires a valid .env file with SSH credentials
// It will only run if the INTEGRATION_TEST environment variable is set to true
func TestCopyFile(t *testing.T) {
	// Skip test if not running integration tests
	// if os.Getenv("INTEGRATION_TEST") != "true" {
	// 	t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run")
	// }

	// Load configuration from .env
	cfg, err := config.Builder().WithEnv("../..").Build()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	opts := Options{
		Host:           cfg.ProxmoxCIHost,
		Username:       cfg.ProxmoxCIUser,
		PrivateKeyPath: cfg.ProxmoxCIPKPath, // Now interpreted as a path to the private key file
		Timeout:        30 * time.Second,
	}

	// Create test content to upload
	content := []byte("This is a test file created by TestCopyFile")

	// Generate a unique remote path for testing
	timestamp := strconv.FormatInt(time.Now().UnixNano(), 10)
	remotePath := filepath.Join(cfg.ProxmoxCIPath, "test-scp-file-"+timestamp+".txt")

	// Test the CopyFile function
	err = CopyFile(opts, content, remotePath)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	t.Logf("Successfully copied content to %s", remotePath)

	// Cleanup and test delete
	err = DeleteFile(opts, remotePath)
	if err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}
	t.Logf("Successfully deleted file at %s", remotePath)
}
