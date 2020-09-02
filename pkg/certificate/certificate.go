package certificate

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/sapcc/git-cert-shim/pkg/util"
	"gopkg.in/yaml.v3"
)

type Certificate struct {
	CommonName string   `yaml:"cn" json:"cn"`
	SANS       []string `yaml:"sans,omitempty" json:"sans,omitempty"`
	OutFolder  string   `yaml:"-" json:"-"`
}

func (c *Certificate) GetName() string {
	commonName := strings.ReplaceAll(c.CommonName, ".", "-")
	return commonName
}

func (c *Certificate) GetSecretName() string {
	return fmt.Sprintf("tls-%s", c.GetName())
}

func ReadCertificateConfig(filepath string) ([]*Certificate, error) {
	fileByte, err := ioutil.ReadFile(filepath)
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
		dirPath, err := util.GetDirPath(filepath)
		if err != nil {
			return nil, err
		}
		certs[idx].OutFolder = dirPath
	}

	return certs, nil
}
