package httpapi

import (
	"os"
	"testing"
)

// skipIfNotIntegrationTest skips the test if INTEGRATION_TEST is not set to true
func skipIfNotIntegrationTest(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run")
	}
}
