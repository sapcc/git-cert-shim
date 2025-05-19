# Build the manager binary
FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.24-alpine AS builder

WORKDIR /workspace

RUN apk update && apk add make

# Copy miscellaneous stuff.
COPY .git/ .git/
COPY hack/ hack/
COPY Makefile Makefile
COPY VERSION VERSION

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source.
COPY main.go main.go
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build the controller.
RUN make build CGO_ENABLED=0

FROM --platform=${BUILDPLATFORM:-linux/amd64} alpine:3.21
LABEL source_repository="https://github.com/sapcc/git-cert-shim"

WORKDIR /

RUN apk upgrade --no-cache --no-progress \
  && apk --update add git less openssh ca-certificates \
  && apk del --no-cache --no-progress apk-tools alpine-keys alpine-release libc-utils

RUN mkdir -p /root/.ssh

# Install SAP CA certificate.
RUN wget -O /usr/local/share/ca-certificates/SAP_Global_Root_CA.crt http://aia.pki.co.sap.com/aia/SAP%20Global%20Root%20CA.crt && update-ca-certificates
RUN echo "StrictHostKeyChecking no" >> /etc/ssh/ssh_config
RUN echo "UserKnownHostsFile /dev/null" >> /etc/ssh/ssh_config

COPY --from=builder /workspace/bin/git-cert-shim .
RUN ["/git-cert-shim", "--version"]
ENTRYPOINT ["/git-cert-shim"]
