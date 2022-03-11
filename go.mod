module github.com/sapcc/git-cert-shim

go 1.16

require (
	github.com/go-logr/logr v0.1.0
	github.com/hashicorp/vault/api v1.0.4
	github.com/jetstack/cert-manager v0.16.1
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.0.0
	go.uber.org/zap v1.10.0
	gopkg.in/yaml.v3 v3.0.0-20190905181640-827449938966
	k8s.io/api v0.18.5
	k8s.io/apimachinery v0.18.5
	k8s.io/client-go v0.18.5
	sigs.k8s.io/controller-runtime v0.5.1-0.20200416234307-5377effd4043
)
