// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"time"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
)

type ControllerOptions struct {
	ConfigFileName,
	Namespace string
	DefaultIssuer           cmmeta.ObjectReference
	RenewCertificatesBefore time.Duration
}

func (co *ControllerOptions) Validate() error {
	if co.ConfigFileName == "" {
		return errors.New("name of file(s) containing certificate configuration not provided")
	}
	if co.DefaultIssuer.Name == "" {
		return errors.New("default-issuer-name missing")
	}
	if co.DefaultIssuer.Kind == "" {
		return errors.New("default-issuer-kind missing")
	}
	if co.DefaultIssuer.Group == "" {
		return errors.New("default-issuer-group missing")
	}
	return nil
}
