# Build the manager binary
FROM golang:1.19 as builder

WORKDIR /workspace

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

FROM alpine:3.17.2
LABEL source_repository="https://github.com/sapcc/git-cert-shim"

WORKDIR /

RUN apk --update add git less openssh && \
    rm -rf /var/lib/apt/lists/* && \
    rm /var/cache/apk/* && git --version

RUN mkdir -p /root/.ssh

# Install SAP CA certificate.
RUN wget -O /usr/local/share/ca-certificates/SAP_Global_Root_CA.crt http://aia.pki.co.sap.com/aia/SAP%20Global%20Root%20CA.crt && update-ca-certificates
RUN ssh-keyscan github.wdf.sap.corp >> /root/.ssh/known_hosts
RUN echo "StrictHostKeyChecking no" >> /etc/ssh/ssh_config

COPY --from=builder /workspace/bin/git-cert-shim .
RUN ["/git-cert-shim", "--version"]
ENTRYPOINT ["/git-cert-shim"]
