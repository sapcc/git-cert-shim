/*/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-logr/logr"
	certmanagerv1alpha2 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	"github.com/sapcc/git-cert-shim/pkg/certificate"
	"github.com/sapcc/git-cert-shim/pkg/config"
	"github.com/sapcc/git-cert-shim/pkg/git"
	"github.com/sapcc/git-cert-shim/pkg/k8sutils"
	"github.com/sapcc/git-cert-shim/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete

type GitController struct {
	ControllerOptions *config.ControllerOptions
	GitOptions        *git.Options
	Log               logr.Logger

	client           client.Client
	scheme           *runtime.Scheme
	repositorySyncer *git.RepositorySyncer
	queue            workqueue.RateLimitingInterface
	wg               sync.WaitGroup
}

func (g *GitController) Start(stop <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer g.queue.ShutDown()
	defer g.wg.Done()
	g.wg.Add(1)

	g.Log.Info("starting controller")

	go wait.Until(g.runWorker, time.Second, stop)

	g.requeueAll()

	ticker := time.NewTicker(g.GitOptions.SyncPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				g.requeueAll()
				g.Log.Info("requeued all certificates", "syncPeriod", g.GitOptions.SyncPeriod)
			case <-stop:
				ticker.Stop()
				return
			}
		}
	}()

	<-stop
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

	logger.Info("ensuring certificate exists in kubernetes", "namespace", g.ControllerOptions.Namespace, "name", cert.GetName())
	c, err := k8sutils.EnsureCertificate(ctx, g.client, g.ControllerOptions.Namespace, cert.GetName(), func(c *certmanagerv1alpha2.Certificate) *certmanagerv1alpha2.Certificate {
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
		return fmt.Errorf("certificate not (yet) ready. re-adding to queue")
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

	certFileName := filepath.Join(cert.OutFolder, fmt.Sprintf("%s.pem", cert.CommonName))
	if err := util.WriteToFileIfNotEmpty(certFileName, certByte); err != nil {
		return err
	}

	keyFileName := filepath.Join(cert.OutFolder, fmt.Sprintf("%s-key.pem", cert.CommonName))
	if err := util.WriteToFileIfNotEmpty(keyFileName, keyByte); err != nil {
		return err
	}

	certRelPath, err := filepath.Rel(cert.OutFolder, certFileName)
	if err != nil {
		return err
	}

	keyRelPath, err := filepath.Rel(cert.OutFolder, keyFileName)
	if err != nil {
		return err
	}

	err = g.repositorySyncer.AddFilesAndCommit(
		fmt.Sprintf("added certificate for %s", cert.CommonName), certRelPath, keyRelPath,
	)
	return err
}

func isCertificateReady(cert *certmanagerv1alpha2.Certificate) bool {
	for _, c := range cert.Status.Conditions {
		if c.Type == certmanagerv1alpha2.CertificateConditionReady {
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

	repoSyncer, err := git.NewRepositorySyncerAndInit(ctrl.Log.WithName("gitsyncer"), g.GitOptions)
	if err != nil {
		return err
	}
	g.repositorySyncer = repoSyncer

	if err := mgr.Add(repoSyncer); err != nil {
		return err
	}

	g.scheme = mgr.GetScheme()
	g.client = mgr.GetClient()
	g.queue = workqueue.NewRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(30*time.Second, 600*time.Second))
	g.ControllerOptions.Namespace = util.GetEnv("NAMESPACE", g.ControllerOptions.Namespace)

	err = mgr.Add(g)
	return err
}
