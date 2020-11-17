/*


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

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	certmanagerv1alpha2 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	uber_zap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/sapcc/git-cert-shim/controllers"
	"github.com/sapcc/git-cert-shim/pkg/config"
	"github.com/sapcc/git-cert-shim/pkg/git"
	"github.com/sapcc/git-cert-shim/pkg/version"
	// +kubebuilder:scaffold:imports
)

const programName = "git-cert-shim"

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = certmanagerv1alpha2.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var (
		metricsAddr string
		isPrintVersionAndExit,
		enableLeaderElection bool
		gitOpts        git.Options
		controllerOpts config.ControllerOptions
		debug          bool
	)

	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&isPrintVersionAndExit, "version", false, "Print version and exit.")

	flag.StringVar(&gitOpts.GithubToken, "github-api-token", "", "Github API token. Alternatively, provide via environment variable GITHUB_API_TOKEN.")
	flag.StringVar(&gitOpts.GithubSSHPrivkeyFilename, "github-ssh-privkey", "", "Github SSH private key filename. Alternatively, provide via environment variable GITHUB_SSH_PRIVKEY.")
	flag.StringVar(&gitOpts.AuthorName, "github-author-name", "certificate-bot", "The name of the author used for commit.")
	flag.StringVar(&gitOpts.AuthorEmail, "github-author-email", "certificate-bot@sap.com", "The email of the author used for commits.")
	flag.StringVar(&gitOpts.RemoteURL, "git-remote-url", "", "The remote URL of the github repository.")
	flag.StringVar(&gitOpts.BranchName, "git-branch-name", "master", "The name of the git branch to synchronize with.")
	flag.DurationVar(&gitOpts.SyncPeriod, "git-sync-period", 15*time.Minute, "The period in which synchronization with the git repository is guaranteed.")
	flag.BoolVar(&gitOpts.IsEnsureEmptyDirectory, "ensure-empty-git-directory", true, "Ensure the creation of an empty directory for the git clone.")
	flag.BoolVar(&gitOpts.DryRun, "dry-run", false, "Do not push to repository.")

	flag.StringVar(&controllerOpts.Namespace, "namespace", "kube-system", "The namespace in which certificate request will be created. Is overwritten by the namespace this controller runs in.")
	flag.StringVar(&controllerOpts.ConfigFileName, "config-file-name", "certificates.yaml", "The file containing the certificate configuration.")
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
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "336042e1.git-cert-shim",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.GitController{
		ControllerOptions: &controllerOpts,
		GitOptions:        &gitOpts,
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
