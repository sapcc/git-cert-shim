// Copyright 2020 SAP SE
// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	vaultapi "github.com/hashicorp/vault/api"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Options struct {
	PushCertificates bool
	UpdateMetaData   bool
	KVEngineName     string
	authRoleID       string
	authSecretID     string
}

type Client struct {
	client  *vaultapi.Client
	Options Options
	Log     logr.Logger

	authMutex      sync.Mutex
	authValidUntil time.Time
}

// Returns (nil, nil) if Vault support is not selected through the respective CLI options.
func NewClientIfSelected(opts Options) (*Client, error) {
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

	// authenticate once immediately to check correctness of credentials
	c := &Client{client: client, Options: opts, authValidUntil: time.Now().Add(-1 * time.Hour)}
	err = c.authenticateIfNecessary()
	if err != nil {
		return nil, err
	}
	c.Log = ctrl.Log.WithName("vaultClient").WithName("controllers").WithName("git")
	return c, nil
}

func (c *Client) authenticateIfNecessary() error {
	c.authMutex.Lock()
	defer c.authMutex.Unlock()

	// use existing token if possible
	if c.authValidUntil.After(time.Now()) {
		return nil
	}

	// perform approle authentication
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

func (c *Client) UpdateCertificate(data CertificateData, certStatus certmanagerv1.CertificateStatus) error {
	err := c.authenticateIfNecessary()
	if err != nil {
		return err
	}

	fullSecretPath := c.secretPath(data.VaultPath)
	payload := map[string]interface{}{ // this exact type is necessary because we do reflect.DeepEqual() below!
		"certificate": string(data.CertBytes),
		"private-key": string(data.KeyBytes),
	}

	// we only want to write the secret and therefore produce a new version when actually necessary
	secret, err := c.client.Logical().Read(fullSecretPath)
	if err != nil {
		c.Log.Error(err, "failed to read secret", "path", fullSecretPath)
		return err
	}

	needsWrite := false
	if secret == nil {
		needsWrite = true // secret does not exist yet
	} else {
		needsWrite = !reflect.DeepEqual(secret.Data["data"], payload)
	}

	if needsWrite && c.Options.PushCertificates {
		_, err := c.client.Logical().Write(fullSecretPath, map[string]interface{}{"data": payload})
		if err != nil {
			return fmt.Errorf("while writing payload to vault: %w", err)
		}

		err = c.patchMetadata(data.VaultPath, certStatus)
		if err != nil {
			return fmt.Errorf("while updating metadata: %w", err)
		}
	} else {
		c.Log.Info("skipping writing to vault", "path", fullSecretPath)
	}

	secretMeta, err := c.client.KVv2(c.Options.KVEngineName).GetMetadata(context.TODO(), data.VaultPath)
	if err != nil {
		c.Log.Error(err, "failed to read secret metadata", "path", fullSecretPath)
		return err
	}

	if c.Options.UpdateMetaData && secretMeta.CustomMetadata["expiry_date"] != fmt.Sprintf("%d-%02d-%02d", certStatus.NotAfter.Year(), certStatus.NotAfter.Month(), certStatus.NotAfter.Day()) {
		err = c.patchMetadata(data.VaultPath, certStatus)
		if err != nil {
			return fmt.Errorf("while updating metadata: %w", err)
		}
	} else {
		c.Log.Info("skipping updated metadata", "path", fullSecretPath, "vaultMetaData", secretMeta.CustomMetadata, "secretMetaData", certStatus)
	}

	return nil
}

func (c *Client) patchMetadata(vaultPath string, certStatus certmanagerv1.CertificateStatus) error {
	customMetadata := map[string]interface{}{
		"accessed_resource":       c.client.Address(),
		"application_criticality": "high",
		"expiry_date":             fmt.Sprintf("%d-%02d-%02d", certStatus.NotAfter.Year(), certStatus.NotAfter.Month(), certStatus.NotAfter.Day()),
		"review_date":             fmt.Sprintf("%d-%02d-%02d", certStatus.RenewalTime.Year(), certStatus.RenewalTime.Month(), certStatus.RenewalTime.Day()),
		"is_privileged":           "false",
		"is_single_factor":        "false",
		"username":                "UNLINKED",
	}

	err := c.client.KVv2(c.Options.KVEngineName).PatchMetadata(context.TODO(), vaultPath, vaultapi.KVMetadataPatchInput{
		CustomMetadata: customMetadata,
	})
	return err
}

func (c *Client) secretPath(filePath string) string {
	return fmt.Sprintf("%s/data/%s", c.Options.KVEngineName, filePath)
}
