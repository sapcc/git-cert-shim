package certificate

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Certificate struct {
	CommonName string   `yaml:"cn" json:"cn"`
	SANS       []string `yaml:"sans,omitempty" json:"sans,omitempty"`
	OutFolder  string   `yaml:"-" json:"-"`
	VaultPath  string   `yaml:"-" json:"-"`
}

func (c *Certificate) GetName() string {
	commonName := strings.ReplaceAll(c.CommonName, ".", "-")
	commonName = strings.ReplaceAll(commonName, "*", "wildcard")
	return commonName
}

func (c *Certificate) GetSecretName() string {
	return fmt.Sprintf("tls-%s", c.GetName())
}

func ReadCertificateConfig(filePath string) ([]*Certificate, error) {
	fileByte, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	type certCfg struct {
		Vault struct {
			PathTemplate string `yaml:"path"`
		} `yaml:"vault"`
		Certificates []*Certificate `yaml:"certificates" json:"certificates"`
	}

	var c certCfg
	if err := yaml.Unmarshal(fileByte, &c); err != nil {
		return nil, err
	}

	vaultPathTpl, err := template.New(filePath + "/vault.path").Parse(c.Vault.PathTemplate)
	if err != nil {
		return nil, err
	}

	certs := c.Certificates
	for idx, c := range certs {
		// Ensure the common name is part of the SANs.
		certs[idx].SANS = checkSANs(c.CommonName, c.SANS)

		// Remember where to store the certificate and key in Git.
		certs[idx].OutFolder = filepath.Dir(filePath)

		// Calculate where to store the certificate and key in Vault.
		var buf bytes.Buffer
		err = vaultPathTpl.Execute(&buf, map[string]interface{}{
			"PathSafeCommonName": strings.ReplaceAll(c.CommonName, "*", "wildcard"),
		})
		if err != nil {
			return nil, fmt.Errorf("while evaluating vault.path template for %q: %w", c.CommonName, err)
		}
		certs[idx].VaultPath = buf.String()
	}

	return certs, nil
}
