package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	gitRemoteURLEnvVarkey      = "GIT_REMOTE_URL"
	gitTokenEnvVarKey          = "GIT_API_TOKEN" //nolint:gosec
	gitSSHPrivkeyFileEnvVarKey = "GIT_SSH_PRIVKEY_FILE"
)

var (
	errGitNoRemote        = errors.New("git remote has no remote configured")
	errGithubNoValidURL   = errors.New("No valid Github URL given")
	sshPrivateKeyFilename = "/root/.ssh/id_rsa"
)

type Options struct {
	// AbsLocalPath is the path of the directory to clone to.
	AbsLocalPath,

	// RemoteURL is the remote URL of the Github repository.
	RemoteURL,

	// BranchName is the name of the git branch. Defaults to master.
	BranchName,

	// AuthorName is the name of the author used for commits.
	AuthorName,

	// AuthorEmail is the email of the author used for commits.
	AuthorEmail,

	// GithubToken is the token for the Github API.
	// Can also be provided via environment variable GITHUB_API_TOKEN.
	GithubToken string

	// GithubSSHPrivkeyFilename is the name of the SSH private key file.
	GithubSSHPrivkeyFilename string

	// IsEnsureEmptyDirectory ensures the local directory is empty before cloning to it.
	IsEnsureEmptyDirectory bool

	// SyncPeriod is the period in which synchronization with the git repository is guaranteed.
	SyncPeriod time.Duration

	// Whether to write certificates into the Git repository (will be false when only pushing certs to other storages)
	PushCertificates bool

	// Do not push to remote repository (but still write to the local Git clone)
	DryRun bool
}

func (o *Options) validate() error {
	if err := o.validateAuth(); err != nil {
		return err
	}

	// Validate remote URL.
	if o.RemoteURL == "" {
		v, ok := os.LookupEnv(gitRemoteURLEnvVarkey)
		if !ok {
			return errGitNoRemote
		}
		o.RemoteURL = v
	}

	if o.GithubToken != "" && !strings.HasPrefix(o.RemoteURL, "https://") {
		return errGithubNoValidURL
	}

	if o.GithubSSHPrivkeyFilename != "" && !strings.HasPrefix(o.RemoteURL, "git") {
		return errGithubNoValidURL
	}

	if o.AbsLocalPath == "" {
		pathParts := strings.SplitAfter(o.RemoteURL, "/")
		tmpDir := pathParts[len(pathParts)-1]
		tmpDir = strings.ReplaceAll(tmpDir, ".", "_")
		o.AbsLocalPath = filepath.Join(os.TempDir(), tmpDir)
	}
	if o.BranchName == "" {
		o.BranchName = "master"
	}
	return nil
}

func (o *Options) validateAuth() error {
	// Attempt to read token from env if unset.
	if ghToken, ok := os.LookupEnv(gitTokenEnvVarKey); ok {
		o.GithubToken = ghToken
	}
	if o.GithubToken != "" {
		fmt.Printf("Using %s from environment for authentication.\n", gitTokenEnvVarKey)
		return nil
	}

	// Attempt to read private key from given file and copy to default location.
	if gitKeyFile, ok := os.LookupEnv(gitSSHPrivkeyFileEnvVarKey); ok {
		fmt.Printf("Using %s from environment to load private key for authentication.\n", gitSSHPrivkeyFileEnvVarKey)
		if err := checkFileExistsAndIsNotEmpty(gitKeyFile); err != nil {
			return err
		}
		if err := copyFile(gitKeyFile, sshPrivateKeyFilename); err != nil {
			return err
		}
	}

	o.GithubSSHPrivkeyFilename = sshPrivateKeyFilename
	err := checkFileExistsAndIsNotEmpty(sshPrivateKeyFilename)
	return err
}

func checkFileExistsAndIsNotEmpty(filename string) error {
	fileByte, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	if len(fileByte) == 0 {
		return fmt.Errorf("file %s is empty", filename)
	}
	return nil
}

func copyFile(src, target string) error {
	srcByte, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	err = os.WriteFile(target, srcByte, 0600)
	return err
}
