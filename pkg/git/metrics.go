// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func init() {
	metrics.Registry.MustRegister(gitSyncErrorTotal)
}

const metricNamespace = "git_cert_shim"

var (
	gitSyncErrorTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   metricNamespace,
		Subsystem:   "git",
		Name:        "synchronization_errors_total",
		Help:        "Counter for git synchronization errors",
		ConstLabels: nil,
	}, []string{"operation"})
)
