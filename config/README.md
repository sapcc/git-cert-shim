<!--
SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company

SPDX-License-Identifier: Apache-2.0
-->

git-cert-shim
-------------

This directory contains the Kustomize base for the git-cert-shim.  

Provide the secrets for your git repository via the [kustomization.yaml](controller/kustomization.yaml).
Eventually, the `git-cert-shim` secret must contain
```
# The github repository to use for the certificates. 
GIT_REMOTE_URL="git@github.com:org/some.git"

# Provide one of the following for authentication.
# Token with read and write access to the git repository.
GIT_API_TOKEN=""

# The private key for SSH authentication.
# Parameter GIT_SSH_PRIVKEY_FILE specifies the absolute path to the file containing the private key.
GIT_SSH_PRIVKEY_FILE="/git-cert-shim.key"
...
files:
  - git-cert-shim.key=key.yaml
```

