package httpapi

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"
)

//go:embed templates/cloud-init-user.tpl
var cloudInitUserTemplate string

// CloudInitParams contains parameters for cloud-init configuration
type CloudInitParams struct {
	Hostname       string
	FQDN           string
	Username       string
	Password       string
	SSHKeys        []string
	ExpirePassword bool
	PackageUpgrade bool
	JoinURL        string
	JoinToken      string
	CACertHash     string
	KubeVersion    string
}

// RenderCloudInit generates a cloud-init configuration from the embedded template
// using the provided parameters
func RenderCloudInit(params CloudInitParams) (string, error) {
	// Parse the embedded template
	tmpl, err := template.New("cloud-init").Parse(cloudInitUserTemplate)
	if err != nil {
		return "", err
	}

	// Execute the template with the provided parameters
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// FormatCicustom formats the cicustom parameter for Proxmox API calls
// Example: user=local:snippets/kube-agent-user-ci-worker.yml
func FormatCicustom(storageName string, path string) string {
	return fmt.Sprintf("user=%s:%s", storageName, path)
}
