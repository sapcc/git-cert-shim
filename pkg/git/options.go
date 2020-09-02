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
	gitRemoteURLEnvVarkey = "GIT_REMOTE_URL"
	ghTokenEnvVarKey      = "GITHUB_TOKEN"
)

var (
	errGitNoRemote   = errors.New("git remote has no remote configured")
	errGithubNoToken = fmt.Errorf("%s environment variable no set", ghTokenEnvVarKey)
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

	// IsEnsureEmptyDirectory ensures the local directory is empty before cloning to it.
	IsEnsureEmptyDirectory bool

	// SyncPeriod is the period in which synchronization with the git repository is guaranteed.
	SyncPeriod time.Duration
}

func (o *Options) validate() error {
	if o.RemoteURL == "" {
		v, ok := os.LookupEnv(gitRemoteURLEnvVarkey)
		if !ok {
			return errGitNoRemote
		}
		o.RemoteURL = v
	}

	if o.BranchName == "" {
		return errors.New("missing git branch name")
	}

	if o.AbsLocalPath == "" {
		pathParts := strings.SplitAfter(o.RemoteURL, "/")
		tmpDir := pathParts[len(pathParts)-1]
		tmpDir = strings.ReplaceAll(tmpDir, ".", "_")
		o.AbsLocalPath = filepath.Join(os.TempDir(), tmpDir)
	}

	if o.GithubToken == "" {
		v, ok := os.LookupEnv(ghTokenEnvVarKey)
		if !ok {
			return errGithubNoToken
		}
		o.GithubToken = v
	}

	return nil
}
