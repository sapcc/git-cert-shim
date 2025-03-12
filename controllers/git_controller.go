// Copyright 2020 SAP SE
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sapcc/git-cert-shim/pkg/certificate"
	"github.com/sapcc/git-cert-shim/pkg/config"
	"github.com/sapcc/git-cert-shim/pkg/git"
	"github.com/sapcc/git-cert-shim/pkg/k8sutils"
	"github.com/sapcc/git-cert-shim/pkg/util"
	"github.com/sapcc/git-cert-shim/pkg/vault"
)

// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps;secrets,verbs=create;get;list;update;patch;watch;delete

type GitController struct {
	ControllerOptions *config.ControllerOptions
	GitOptions        *git.Options
	VaultClient       *vault.Client
	Log               logr.Logger
	client            client.Client
	scheme            *runtime.Scheme
	repositorySyncer  *git.RepositorySyncer
	queue             workqueue.TypedRateLimitingInterface[interface{}]
	wg                sync.WaitGroup
	mtx               sync.Mutex
}

func (g *GitController) Start(ctx context.Context) error {
	defer utilruntime.HandleCrash()
	defer g.queue.ShutDown()
	defer g.wg.Done()
	g.wg.Add(1)

	g.Log.Info("starting controller")

	go wait.Until(g.runWorker, time.Second, ctx.Done())

	g.requeueAll()
	go func() {
		ticker := time.NewTicker(g.GitOptions.SyncPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				g.requeueAll()
				g.Log.Info("requeued all certificates", "syncPeriod", g.GitOptions.SyncPeriod)
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
	g.Log.Info("stopping controller")
	return nil
}

func (g *GitController) runWorker() {
	for g.processNextWorkItem() {
	}
}

func (g *GitController) processNextWorkItem() bool {
	itm, shutdown := g.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer g.queue.Done(obj)

		c, ok := obj.(*certificate.Certificate)
		if !ok {
			g.queue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected certificate in queue but got %#v", obj))
			return nil
		}

		if err := g.checkCertificate(c); err != nil {
			g.queue.AddRateLimited(c)
			return fmt.Errorf("error syncing certificate for host %s: %s, requeuing", c.CommonName, err.Error())
		}

		g.queue.Forget(c)
		g.Log.Info("successfully synced certificate", "host", c.CommonName)
		return nil
	}(itm)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (g *GitController) checkCertificate(cert *certificate.Certificate) error {
	ctx := context.Background()
	logger := g.Log.WithValues("host", cert.CommonName)

	logger.Info("ensuring certificate exists in cluster", "namespace", g.ControllerOptions.Namespace, "name", cert.GetName())
	c, err := k8sutils.EnsureCertificate(ctx, g.client, g.ControllerOptions.Namespace, cert.GetName(), func(c *certmanagerv1.Certificate) *certmanagerv1.Certificate {
		c.Spec.IssuerRef = g.ControllerOptions.DefaultIssuer
		c.Spec.CommonName = cert.CommonName
		c.Spec.DNSNames = cert.SANS
		c.Spec.SecretName = cert.GetSecretName()
		c.Spec.RenewBefore = &metav1.Duration{Duration: g.ControllerOptions.RenewCertificatesBefore}
		return c
	})
	if err != nil {
		logger.Error(err, "failed to ensure certificate", "namespace", g.ControllerOptions.Namespace, "name", cert.GetName())
		return err
	}

	// If the certmanager.certificate is not ready, we abort here and check again later.
	// Once it is ready, the secret contains the tls certificate and private key.
	if !isCertificateReady(c) {
		return errors.New("certificate not (yet) ready. re-adding to queue")
	}

	tlsSecret, err := k8sutils.GetSecret(ctx, g.client, g.ControllerOptions.Namespace, cert.GetSecretName())
	if err != nil {
		logger.Error(err, "failed to get secret", "namespace", g.ControllerOptions.Namespace, "name", cert.GetSecretName())
		return err
	}

	_, certByte, keyByte, err := certificate.ExtractCAAndCertificateAndPrivateKeyFromSecret(tlsSecret)
	if err != nil {
		logger.Error(err, "failed to extract certificates and key from secret", "namespace", g.ControllerOptions.Namespace, "name", cert.GetSecretName())
		return err
	}

	if g.VaultClient != nil && g.VaultClient.Options.PushCertificates {
		err := g.VaultClient.UpdateCertificate(vault.CertificateData{
			VaultPath: cert.VaultPath,
			CertBytes: certByte,
			KeyBytes:  keyByte,
		}, c.Status)
		if err != nil {
			logger.Error(err, "failed to write certificate to Vault", "namespace", g.ControllerOptions.Namespace, "name", cert.GetSecretName())
			return err
		}
	}

	if g.GitOptions.PushCertificates {
		// Wait for syncer to finish
		g.mtx.Lock()
		defer g.mtx.Unlock()

		certFileName := filepath.Join(cert.OutFolder, cert.CommonName+".pem")
		certFileName = strings.ReplaceAll(certFileName, "*", "wildcard")
		if err := util.WriteToFileIfNotEmpty(certFileName, certByte); err != nil {
			return err
		}

		keyFileName := filepath.Join(cert.OutFolder, cert.CommonName+"-key.pem")
		keyFileName = strings.ReplaceAll(keyFileName, "*", "wildcard")
		if err := util.WriteToFileIfNotEmpty(keyFileName, keyByte); err != nil {
			return err
		}

		err = g.repositorySyncer.AddFilesAndCommit(
			"added certificate for "+cert.CommonName, certFileName, keyFileName,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func isCertificateReady(cert *certmanagerv1.Certificate) bool {
	for _, c := range cert.Status.Conditions {
		if c.Type == certmanagerv1.CertificateConditionReady {
			return c.Status == cmmeta.ConditionTrue
		}
	}

	return false
}

func (g *GitController) requeueAll() {
	allFiles, err := util.FindFilesInPath(g.GitOptions.AbsLocalPath, g.ControllerOptions.ConfigFileName)
	if err != nil {
		g.Log.Error(err, "failed to recursively find files in path", "path", g.GitOptions.AbsLocalPath, "filename", g.ControllerOptions.ConfigFileName)
		return
	}

	for _, file := range allFiles {
		certs, err := certificate.ReadCertificateConfig(file)
		if err != nil {
			g.Log.Error(err, "failed to read configuration", "file", file)
			continue
		}

		g.enqueueCertificates(certs)
	}
}

func (g *GitController) enqueueCertificates(certs []*certificate.Certificate) {
	for _, c := range certs {
		g.queue.AddRateLimited(c)
	}
}

func (g *GitController) SetupWithManager(mgr ctrl.Manager) error {
	if err := g.ControllerOptions.Validate(); err != nil {
		return err
	}

	g.mtx = sync.Mutex{}
	repoSyncer, err := git.NewRepositorySyncerAndInit(ctrl.Log.WithName("gitsyncer"), g.GitOptions, &g.mtx)
	if err != nil {
		return err
	}
	g.repositorySyncer = repoSyncer

	if err := mgr.Add(repoSyncer); err != nil {
		return err
	}

	g.scheme = mgr.GetScheme()
	g.client = mgr.GetClient()
	g.queue = workqueue.NewTypedRateLimitingQueue(workqueue.NewTypedItemExponentialFailureRateLimiter[interface{}](30*time.Second, 600*time.Second))
	g.ControllerOptions.Namespace = util.GetEnv("NAMESPACE", g.ControllerOptions.Namespace)

	if err := mgr.Add(g); err != nil {
		return err
	}
	return nil
}
