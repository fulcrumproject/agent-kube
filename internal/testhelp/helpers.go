package testhelp

import (
	"os"
	"testing"
	"time"
)

// skipIfNotIntegrationTest skips the test if INTEGRATION_TEST is not set to true
func SkipIfNotIntegrationTest(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run")
	}
}

// GenerateTestVMID creates a test VM ID that's unlikely to conflict with existing VMs
func GenerateTestVMID() int {
	timestamp := time.Now().Unix()
	return 900_000 + int(timestamp%10000) // VM ID in range 900000-909999
}
