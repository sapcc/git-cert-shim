package git

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	gitRemoteURLEnvVarkey = "GIT_REMOTE_URL"
	ghTokenEnvVarKey      = "GITHUB_API_TOKEN"
	ghSshPrivkeyEnvVarKey = "GITHUB_SSH_PRIVKEY"
)

var (
	errGitNoRemote           = errors.New("git remote has no remote configured")
	errGithubNoAuth          = errors.New("No authentication method found")
	errGithubNoValidURL      = errors.New("No valid Github URL given")
	tmpSshPrivateKeyFilename = "/git-cert-shim-ssh-privatekey"
)

type Options struct {
	// AbsLocalPath is the path of the directory to clone to.
	AbsLocalPath,

	// RemoteURL is the remote URL of the github repository.
	RemoteURL,

	// BranchName is the name of the git branch.
	BranchName,

	// AuthorName is the name of the author used for commits.
	AuthorName,

	// AuthorEmail is the email of the author used for commits.
	AuthorEmail,

	// GithubToken is the token for the Github API.
	// Can also be provided via environment variable GITHUB_API_TOKEN.
	GithubToken string

	// Github SSH private key file
	GithubSSHPrivkeyFilename string

	// IsEnsureEmptyDirectory ensures the local directory is empty before cloning to it.
	IsEnsureEmptyDirectory bool

	// SyncPeriod is the period in which synchronization with the git repository is guaranteed.
	SyncPeriod time.Duration
}

func (o *Options) validate() error {
	if o.GithubToken == "" && o.GithubSSHPrivkeyFilename == "" {
		v, ok1 := os.LookupEnv(ghTokenEnvVarKey)
		if ok1 && v != "" {
			o.GithubToken = v
		}

		v, ok2 := os.LookupEnv(ghSshPrivkeyEnvVarKey)
		if ok2 && v != "" {
			err := ioutil.WriteFile(tmpSshPrivateKeyFilename, []byte(v), 0600)
			if err != nil {
				return err
			}
			o.GithubSSHPrivkeyFilename = tmpSshPrivateKeyFilename
		}

		if o.GithubToken == "" && o.GithubSSHPrivkeyFilename == "" {
			return errGithubNoAuth
		}
	}

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

	return nil
}
