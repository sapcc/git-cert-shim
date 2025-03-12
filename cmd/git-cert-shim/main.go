// Copyright 2020 SAP SE
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint:gosec
	"os"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	uber_zap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/sapcc/git-cert-shim/controllers"
	"github.com/sapcc/git-cert-shim/pkg/config"
	"github.com/sapcc/git-cert-shim/pkg/git"
	"github.com/sapcc/git-cert-shim/pkg/vault"
	"github.com/sapcc/git-cert-shim/pkg/version"
	// +kubebuilder:scaffold:imports
)

const programName = "git-cert-shim"

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		profilerAddr,
		metricsAddr string
		isPrintVersionAndExit,
		enableLeaderElection bool
		gitOpts        git.Options
		vaultOpts      vault.Options
		controllerOpts config.ControllerOptions
		debug          bool
	)

	flag.StringVar(&profilerAddr, "profiler-addr", "localhost:6060", "The address to expose pprof profiler on.")
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&isPrintVersionAndExit, "version", false, "Print version and exit.")

	flag.StringVar(&gitOpts.GithubToken, "github-api-token", "", "Github API token. Alternatively, provide via environment variable GIT_API_TOKEN.")
	flag.StringVar(&gitOpts.GithubSSHPrivkeyFilename, "github-ssh-privkey-file", "", "Github SSH private key filename. Alternatively, provide via environment variable GIT_SSH_PRIVKEY_FILE.")
	flag.StringVar(&gitOpts.AuthorName, "github-author-name", "certificate-bot", "The name of the author used for commit.")
	flag.StringVar(&gitOpts.AuthorEmail, "github-author-email", "certificate-bot@sap.com", "The email of the author used for commits.")
	flag.StringVar(&gitOpts.RemoteURL, "git-remote-url", "", "The remote URL of the github repository.")
	flag.StringVar(&gitOpts.BranchName, "git-branch-name", "master", "The name of the git branch to synchronize with.")
	flag.DurationVar(&gitOpts.SyncPeriod, "git-sync-period", 15*time.Minute, "The period in which synchronization with the git repository is guaranteed.")
	flag.BoolVar(&gitOpts.IsEnsureEmptyDirectory, "ensure-empty-git-directory", true, "Ensure the creation of an empty directory for the git clone.")
	flag.BoolVar(&gitOpts.PushCertificates, "git-push-certs", true, "Whether to write certificates into the Git repository. Set to false if you want to push to Vault only.")
	flag.BoolVar(&gitOpts.DryRun, "dry-run", false, "Write certificates into local Git clone, but do not push them.")

	flag.BoolVar(&vaultOpts.PushCertificates, "vault-push-certs", false, "Whether to write certificates into a Vault KV engine. If set to true, Vault credentials must be given in environment variables (VAULT_ADDR and VAULT_ROLE_ID+VAULT_SECRET_ID for approle auth.)")
	flag.StringVar(&vaultOpts.KVEngineName, "vault-kv-engine", "secrets", "Name of KV engine where certificates will be stored in Vault.")

	flag.StringVar(&controllerOpts.Namespace, "namespace", "kube-system", "The namespace in which certificate request will be created. Is overwritten by the namespace this controller runs in.")
	flag.StringVar(&controllerOpts.ConfigFileName, "config-file-name", "git-cert-shim.yaml", "The file containing the certificate configuration.")
	flag.StringVar(&controllerOpts.DefaultIssuer.Name, "default-issuer-name", "", "The name of the issuer used to sign certificate requests.")
	flag.StringVar(&controllerOpts.DefaultIssuer.Kind, "default-issuer-kind", "", "The kind of the issuer used to sign certificate requests.")
	flag.StringVar(&controllerOpts.DefaultIssuer.Group, "default-issuer-group", "", "The group of the issuer used to sign certificate requests.")
	flag.DurationVar(&controllerOpts.RenewCertificatesBefore, "renew-certificates-before", 720*time.Hour, "*Warning*: Only allows min, hour. Trigger renewal of the certificate if they would expire in less than the configured duration.")

	flag.BoolVar(&debug, "debug", false, "Set debug log level.")

	flag.Parse()
	if isPrintVersionAndExit {
		fmt.Println(version.Print(programName))
		os.Exit(0)
	}

	level := uber_zap.NewAtomicLevelAt(zapcore.InfoLevel)
	if debug {
		level = uber_zap.NewAtomicLevelAt(zapcore.DebugLevel)
	}
	ctrl.SetLogger(zap.New(zap.UseDevMode(true), zap.Level(&level)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsAddr,
		},
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "336042e1.git-cert-shim",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		server := &http.Server{Addr: profilerAddr}
		go func() {
			<-ctx.Done()
			server.Close()
		}()
		setupLog.Info("Starting /debug/pprof server", "listen", profilerAddr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
		return nil
	})); err != nil {
		setupLog.Error(err, "unable to create pprof profiler")
		os.Exit(1)
	}

	vaultClient, err := vault.NewClientIfSelected(vaultOpts)
	if err != nil {
		setupLog.Error(err, "unable to create Vault client")
		os.Exit(1)
	}

	if err = (&controllers.GitController{
		ControllerOptions: &controllerOpts,
		GitOptions:        &gitOpts,
		VaultClient:       vaultClient,
		Log:               ctrl.Log.WithName("controllers").WithName("git"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "git")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
