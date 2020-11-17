# git-cert-shim

Automatic management of certificates not related to Kubernetes resources via a github repository.

The controller watches the configured github repository for files containing certificate configurations and
creates cert-manager resources in the current Kubernetes cluster. Once the certificate was issued or was renewed, it is pushed to the github repository.

## Usage & Configuration

Mandatory configuration:
```
// The file containing the certificate configuration. (default "certificates.yaml")
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
// Github API token. Alternatively, provide via environment variable GITHUB_API_TOKEN.
--github-api-token

// Github SSH private key filename. Alternatively, provide via environment variable GITHUB_SSH_PRIVKEY.
--github-ssh-privkey
```

A `certificates.yaml` might look as follows
```
certificates:
  - cn: some.thing.tld
  - cn: foo.bar.tld
    sans:
      - baz.bar.tld
```

The resulting files containing the certificate and private key will be named after the certificates common name, e.g. `some-thing-tld.pem`, `some-thing-tld-key.pem` and are stored in the same folder as the configuration.
