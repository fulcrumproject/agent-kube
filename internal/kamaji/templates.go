package kamaji

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"text/template"

	"fulcrumproject.org/kube-agent/internal/agent"
)

// Template types
type Template string

//go:embed template_tcp.gotmpl
var tcpTemplate Template

//go:embed template_jointoken.gotmpl
var joinTokenTemplate Template

// TCPTemplateParams contains parameters for generating a TenantControlPlane JSON payload
type TCPTemplateParams struct {
	Name     string
	Version  string
	Replicas int
}

// JoinTokenTemplateParams contains parameters for generating a join token JSON payload
type JoinTokenTemplateParams struct {
	TokenID        string
	TokenSecret    string
	ExpirationTime string
}

// generateTCPPayload generates a JSON payload for creating a TenantControlPlane
func generateTCPPayload(params TCPTemplateParams) (*agent.TCPResponse, error) {
	// Parse the template
	tmpl, err := template.New("tcp").Parse(string(tcpTemplate))
	if err != nil {
		return nil, fmt.Errorf("failed to parse TenantControlPlane template: %w", err)
	}

	// Execute the template with the provided parameters
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return nil, fmt.Errorf("failed to execute TenantControlPlane template: %w", err)
	}

	// Parse the generated JSON into the TCP response struct
	var tcpResponse agent.TCPResponse
	if err := json.Unmarshal(buf.Bytes(), &tcpResponse); err != nil {
		return nil, fmt.Errorf("failed to parse TenantControlPlane JSON: %w", err)
	}

	return &tcpResponse, nil
}

// generateJoinTokenPayload generates a JSON payload for creating a join token
func generateJoinTokenPayload(params JoinTokenTemplateParams) ([]byte, error) {
	// Parse the template
	tmpl, err := template.New("jointoken").Parse(string(joinTokenTemplate))
	if err != nil {
		return nil, fmt.Errorf("failed to parse join token template: %w", err)
	}

	// Execute the template with the provided parameters
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return nil, fmt.Errorf("failed to execute join token template: %w", err)
	}

	return buf.Bytes(), nil
}
