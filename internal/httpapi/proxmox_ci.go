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

// CloudInitFile represents a cloud-init file to be uploaded to Proxmox
type CloudInitFile struct {
	NodeName    string
	StorageName string
	FileName    string
	Content     string
	Path        string // The formatted path for cicustom parameter
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

// GenerateCloudInitFile creates a CloudInitFile with the given parameters
func GenerateCloudInitFile(nodeName string, storageName string, vmName string, params CloudInitParams) (*CloudInitFile, error) {
	// Generate cloud-init content
	content, err := RenderCloudInit(params)
	if err != nil {
		return nil, fmt.Errorf("failed to render cloud-init: %w", err)
	}

	// Generate filename based on VM name
	fileName := fmt.Sprintf("kube-agent-user-ci-%s.yml", vmName)

	// Generate the formatted path for cicustom parameter
	cloudInitPath := fmt.Sprintf("%s:user=snippets/%s", storageName, fileName)

	return &CloudInitFile{
		NodeName:    nodeName,
		StorageName: storageName,
		FileName:    fileName,
		Content:     content,
		Path:        cloudInitPath,
	}, nil
}
