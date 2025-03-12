# Image URL to use all building/pushing image targets
IMG ?= keppel.eu-de-1.cloud.sap/ccloud/git-cert-shim

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

## Location to install dependencies an GO binaries
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)
## Tool Binaries
GOLINT ?= $(LOCALBIN)/golangci-lint
## Tool Versions
GOLINT_VERSION ?= 1.64.6
GINKGOLINTER_VERSION ?= 0.19.1
CONTROLLER_GEN_VERSION ?= 0.16.5

all: build
build-all: build

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
	$(CONTROLLER_GEN) rbac:roleName=git-cert-shim-role webhook paths="./..." output:rbac:artifacts:config=config/rbac

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
	docker build . --platform linux/amd64 -t ${IMG}:${VERSION}

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
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CONTROLLER_GEN_VERSION)
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

.PHONY: lint
lint: golint
	$(GOLINT) run -v --timeout 5m

.PHONY: golint
golint: $(GOLINT)
$(GOLINT): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@v$(GOLINT_VERSION)
	GOBIN=$(LOCALBIN) go install github.com/nunnatsa/ginkgolinter/cmd/ginkgolinter@v$(GINKGOLINTER_VERSION)

install-go-licence-detector:
	@if ! hash go-licence-detector 2>/dev/null; then printf "\e[1;36m>> Installing go-licence-detector (this may take a while)...\e[0m\n"; go install go.elastic.co/go-licence-detector@latest; fi

check-dependency-licenses: install-go-licence-detector
	@printf "\e[1;36m>> go-licence-detector\e[0m\n"
	@go list -m -mod=readonly -json all | go-licence-detector -includeIndirect -rules .license-scan-rules.json -overrides .license-scan-overrides.jsonl

GO_TESTENV =
GO_BUILDFLAGS =
GO_LDFLAGS =
# which packages to test with test runner
GO_TESTPKGS := $(shell go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./...)
ifeq ($(GO_TESTPKGS),)
GO_TESTPKGS := ./...
endif
# which packages to measure coverage for
GO_COVERPKGS := $(shell go list ./...)
# to get around weird Makefile syntax restrictions, we need variables containing nothing, a space and comma
null :=
space := $(null) $(null)
comma := ,

build/cover.out: build
	test -d build || mkdir build
	@printf "\e[1;36m>> Running tests\e[0m\n"
	@env $(GO_TESTENV) go test -shuffle=on -p 1 -coverprofile=$@ $(GO_BUILDFLAGS) -ldflags "-s -w -X github.com/sapcc/git-cert-shim/pkg/version.Revision=$(GIT_REVISION) -X github.com/sapcc/git-cert-shim/pkg/version.Branch=$(GIT_BRANCH) -X github.com/sapcc/git-cert-shim/pkg/version.BuildDate=$(BUILD_DATE) -X github.com/sapcc/git-cert-shim/pkg/version.Version=$(VERSION)" -covermode=count -coverpkg=$(subst $(space),$(comma),$(GO_COVERPKGS)) $(GO_TESTPKGS)
