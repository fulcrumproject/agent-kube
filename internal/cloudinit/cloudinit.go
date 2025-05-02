package cloudinit

import (
	"bytes"
	_ "embed"
	"text/template"
)

type Template string

//go:embed cloudinit.gotmpl
var Templ Template

//go:embed cloudinit_test.gotmpl
var TestTempl Template

// Params contains parameters for cloud-init configuration
type Params struct {
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

// Generate generates a cloud-init configuration from the embedded template
// using the provided parameters
func Generate(templ Template, params Params) (string, error) {
	tmpl, err := template.New("cloudinit").Parse(string(templ))
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
