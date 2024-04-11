/*******************************************************************************
*
* Copyright 2022 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package vault

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"sync"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
)

type Options struct {
	PushCertificates bool
	KVEngineName     string
	authRoleID       string
	authSecretID     string
}

type Client struct {
	client  *vaultapi.Client
	Options Options

	authMutex      sync.Mutex
	authValidUntil time.Time
}

// Returns (nil, nil) if Vault support is not selected through the respective CLI options.
func NewClientIfSelected(opts Options) (*Client, error) {
	if !opts.PushCertificates {
		return nil, nil
	}
	if opts.KVEngineName == "" {
		return nil, errors.New("no value given for --vault-kv-engine")
	}

	if os.Getenv("VAULT_ADDR") == "" { //NOTE: VAULT_ADDR is later read by vaultapi.DefaultConfig()
		return nil, errors.New("missing required environment variable: VAULT_ADDR")
	}
	opts.authRoleID = os.Getenv("VAULT_ROLE_ID")
	if opts.authRoleID == "" {
		return nil, errors.New("missing required environment variable: VAULT_ROLE_ID")
	}
	opts.authSecretID = os.Getenv("VAULT_SECRET_ID")
	if opts.authSecretID == "" {
		return nil, errors.New("missing required environment variable: VAULT_SECRET_ID")
	}

	client, err := vaultapi.NewClient(vaultapi.DefaultConfig())
	if err != nil {
		return nil, err
	}

	//authenticate once immediately to check correctness of credentials
	c := &Client{client: client, Options: opts, authValidUntil: time.Now().Add(-1 * time.Hour)}
	err = c.authenticateIfNecessary()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) authenticateIfNecessary() error {
	c.authMutex.Lock()
	defer c.authMutex.Unlock()

	//use existing token if possible
	if c.authValidUntil.After(time.Now()) {
		return nil
	}

	//perform approle authentication
	resp, err := c.client.Logical().Write("auth/approle/login", map[string]interface{}{
		"role_id":   c.Options.authRoleID,
		"secret_id": c.Options.authSecretID,
	})
	if err != nil {
		return fmt.Errorf("while obtaining approle token: %w", err)
	}
	c.client.SetToken(resp.Auth.ClientToken)
	c.authValidUntil = time.Now().Add(time.Duration(resp.Auth.LeaseDuration) * time.Second)

	return nil
}

type CertificateData struct {
	VaultPath string
	CertBytes []byte
	KeyBytes  []byte
}

func (c *Client) UpdateCertificate(data CertificateData) error {
	err := c.authenticateIfNecessary()
	if err != nil {
		return err
	}

	fullSecretPath := path.Join(c.Options.KVEngineName, "data", data.VaultPath)
	payload := map[string]interface{}{ // this exact type is necessary because we do reflect.DeepEqual() below!
		"certificate": string(data.CertBytes),
		"private-key": string(data.KeyBytes),
	}

	//we only want to write the secret and therefore produce a new version when actually necessary
	secret, err := c.client.Logical().Read(fullSecretPath)
	if err != nil {
		return err
	}
	needsWrite := false
	if secret == nil {
		needsWrite = true //secret does not exist yet
	} else {
		needsWrite = !reflect.DeepEqual(secret.Data["data"], payload)
	}

	if needsWrite {
		_, err := c.client.Logical().Write(fullSecretPath, map[string]interface{}{"data": payload})
		if err != nil {
			return fmt.Errorf("while wrinting payload to vault: %w", err)
		}
		err = c.patchMetadata(fullSecretPath)
		return err
	}
	return nil
}

func (c *Client) patchMetadata(fullSecretPath string) error {
	t := time.Now().Add(365 * 24 * time.Hour)
	date := fmt.Sprintf("%d-%02d-%02d", t.Year(), t.Month(), t.Day())
	customMetadata := map[string]interface{}{
		"accessed_resource":       c.client.Address(),
		"application_criticality": "high",
		"expiry_date":             date,
		"review_date":             date,
		"is_privileged":           "false",
		"is_single_factor":        "false",
		"username":                "UNLINKED",
	}

	err := c.client.KVv2(fullSecretPath).PatchMetadata(context.TODO(), fullSecretPath, vaultapi.KVMetadataPatchInput{
		CustomMetadata: customMetadata,
	})
	return err
}
