# Image URL to use all building/pushing image targets
IMG ?= keppel.eu-de-1.cloud.sap/ccloud/git-cert-shim
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: build

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build git-cert-shim binary
build: BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
build: GIT_REVISION=$(shell git rev-parse --short HEAD)
build: GIT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
build: VERSION=$(shell cat VERSION)
build: generate fmt vet
	go build\
	  -ldflags "-s -w -X github.com/sapcc/git-cert-shim/pkg/version.Revision=$(GIT_REVISION) -X github.com/sapcc/git-cert-shim/pkg/version.Branch=$(GIT_BRANCH) -X github.com/sapcc/git-cert-shim/pkg/version.BuildDate=$(BUILD_DATE) -X github.com/sapcc/git-cert-shim/pkg/version.Version=$(VERSION)"\
      -o bin/git-cert-shim main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: VERSION=$(shell cat VERSION)
deploy: manifests
	@if [ ! -f deploy-with-kustomize ]; then echo "ERROR: The deployment in CCloud is now managed with Helm! Refusing to deploy unless you 'touch ./deploy-with-kustomize'."; false; fi
	cd config/controller && kustomize edit set image git-cert-shim=${IMG}:${VERSION}
	kustomize build config | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=git-cert-shim-role webhook paths="./..." output:rbac:artifacts:config=config/rbac

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: VERSION=$(shell cat VERSION)
docker-build:
	docker build . -t ${IMG}:${VERSION}

# Push the docker image
docker-push: VERSION=$(shell cat VERSION)
docker-push:
	docker push ${IMG}:${VERSION} && \
    docker tag ${IMG}:${VERSION} ${IMG}:latest && \
    docker push ${IMG}:latest

git-push-tag: VERSION=$(shell cat VERSION)
git-push-tag:
	git push origin ${VERSION}

git-tag-release: VERSION=$(shell cat VERSION)
git-tag-release: check-release-version
	git tag --annotate ${VERSION} --message "git-cert-shim ${VERSION}"

check-release-version: VERSION=$(shell cat VERSION)
check-release-version:
	if test x$$(git tag --list ${VERSION}) != x; \
	then \
		echo "Tag [${VERSION}] already exists. Please check the working copy."; git diff . ; exit 1;\
	fi

set-image: VERSION=$(shell cat VERSION)
set-image:
	cd config/controller && kustomize edit set image git-cert-shim=${IMG}:${VERSION}
	git commit -am "set git-cert-shim image to ${VERSION}"

release: VERSION=$(shell cat VERSION)
release: git-tag-release set-image git-push-tag docker-build docker-push

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
