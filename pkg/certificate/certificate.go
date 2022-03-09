package certificate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Certificate struct {
	CommonName string   `yaml:"cn" json:"cn"`
	SANS       []string `yaml:"sans,omitempty" json:"sans,omitempty"`
	OutFolder  string   `yaml:"-" json:"-"`
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
		Certificates []*Certificate `yaml:"certificates" json:"certificates"`
	}

	var c certCfg
	if err := yaml.Unmarshal(fileByte, &c); err != nil {
		return nil, err
	}

	certs := c.Certificates
	for idx, c := range certs {
		// Ensure the common name is part of the SANs.
		certs[idx].SANS = checkSANs(c.CommonName, c.SANS)

		// Remember where to store the certificate and key.
		certs[idx].OutFolder = filepath.Dir(filePath)
	}

	return certs, nil
}
