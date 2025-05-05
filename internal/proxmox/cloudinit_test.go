package proxmox

import (
	"strings"
	"testing"
)

func TestRenderCloudInit(t *testing.T) {
	params := CloudInitParams{
		Hostname:       "test-worker-node",
		FQDN:           "test-worker-node",
		Username:       "ubuntu",
		Password:       "ubuntu",
		SSHKeys:        []string{"ssh-rsa AAAAB3NzaC1yc2EAAAA... test@example.com"},
		ExpirePassword: false,
		PackageUpgrade: true,
		JoinURL:        "172.30.232.66:6443",
		JoinToken:      "08f863.6357ad0f550c8e04",
		CACertHash:     "sha256:1992ff0cf2bc550fd67ad3238e1355a47ce6b2f32a009f433139b5985066db54",
		KubeVersion:    "v1.30.5",
	}

	result, err := GenerateCloudInit(CloudInitTestTempl, params)
	if err != nil {
		t.Fatalf("Failed to render cloud-init: %v", err)
	}

	// Verify that the output contains key elements
	expectedStrings := []string{
		"#cloud-config",
		"hostname: test-worker-node",
		"user: ubuntu",
		"password: ubuntu",
		"package_upgrade: true",
		"JOIN_URL=172.30.232.66:6443",
		"JOIN_TOKEN=08f863.6357ad0f550c8e04",
		"JOIN_TOKEN_CACERT_HASH=sha256:1992ff0cf2bc550fd67ad3238e1355a47ce6b2f32a009f433139b5985066db54",
		"KUBERNETES_VERSION=v1.30.5",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected rendered cloud-init to contain '%s', but it doesn't.\nGot: %s", expected, result)
		}
	}
}
