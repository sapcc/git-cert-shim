// Copyright 2020 SAP SE
// SPDX-License-Identifier: Apache-2.0

package certificate

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	caCrt  = "ca.crt"
	tlsCrt = "tls.crt"
	tlsKey = "tls.key"
)

func ExtractCAAndCertificateAndPrivateKeyFromSecret(tlsSecret *corev1.Secret) (ca, cert, key []byte, err error) {
	//nolint:gosimple
	if tlsSecret.Data == nil || len(tlsSecret.Data) == 0 {
		return nil, nil, nil, errors.New("secret is empty")
	}

	// Optional.
	ca = tlsSecret.Data[caCrt]

	cert, ok := tlsSecret.Data[tlsCrt]
	if !ok {
		return nil, nil, nil, fmt.Errorf("%s missing in secret data", tlsCrt)
	}

	key, ok = tlsSecret.Data[tlsKey]
	if !ok {
		return nil, nil, nil, fmt.Errorf("%s missing in secret data", tlsKey)
	}

	return ca, cert, key, nil
}
