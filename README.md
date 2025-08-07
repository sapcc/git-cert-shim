<!--
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company

SPDX-License-Identifier: Apache-2.0
-->

# git-cert-shim

The git-cert-shim extends the [cert-manager](https://github.com/jetstack/cert-manager) and enables 
automating management of certificates configured via a Github repository.

The controller watches the configured Github repository for files containing certificate configurations and
manages [cert-manager resources](https://cert-manager.io/docs/usage/certificate) in the current Kubernetes cluster.  
Once the certificate was issued or renewed, it is kept in sync with the github repository.

## Usage & Configuration

Mandatory configuration:
```
// The file containing the certificate configuration. (default "git-cert-shim.yaml")
--config-file-name

// The remote URL of the github repository.
--git-remote-url

// The group of the issuer used to sign certificate requests.
--default-issuer-group string

// The kind of the issuer used to sign certificate requests.
--default-issuer-kind string

// The name of the issuer used to sign certificate requests.
--default-issuer-name string

// Trigger renewal of the certificate if they would expire in less than the configured duration. 
// *Warning*: Only allows min, hour.  (default 720h0m0s)
--renew-certificates-before duration
```

And choose one authentication method:
```
// Github API token. Alternatively, provide via environment variable GIT_API_TOKEN.
--git-api-token

// Github SSH private key filename. Alternatively, provide via environment variable GIT_SSH_PRIVKEY_FILE.
--git-ssh-privkey-file
```

A `git-cert-shim.yaml` might look as follows
```
certificates:
  - cn: some.thing.tld
  - cn: foo.bar.tld
    sans:
      - baz.bar.tld
```

The resulting files containing the certificate and private key will be named after the certificates common name, e.g. `some-thing-tld.pem`, `some-thing-tld-key.pem` and are stored in the same folder as the configuration.

# Installation

See the provided [kustomize base](config) and provide the required secrets.  
Run `make install` to deploy the git-cert-shim to the current cluster.
