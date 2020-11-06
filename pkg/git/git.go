package git

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/sapcc/git-cert-shim/pkg/util"
)

type Git struct {
	*Options
	*command
}

func NewGit(opts *Options) (*Git, error) {
	var err error

	if err := opts.validate(); err != nil {
		return nil, err
	}

	if !filepath.IsAbs(opts.AbsLocalPath) {
		return nil, fmt.Errorf("requires an absolute path. cannot use: %s", opts.AbsLocalPath)
	}

	if err := util.EnsureDir(opts.AbsLocalPath, opts.IsEnsureEmptyDirectory); err != nil {
		return nil, errors.Wrapf(err, "failed to get or create path %s", opts.AbsLocalPath)
	}

	var cmd *command

	if opts.GithubSSHPrivkeyFilename != "" {
		//cmd, err = newCommand("ssh-agent", "sh", "-c", "ssh-add", opts.GithubSSHPrivkeyFilename, ";", "git", "-C", opts.AbsLocalPath)
		cmd, err = newCommand("/git-wrapper.sh", opts.GithubSSHPrivkeyFilename, opts.AbsLocalPath)
	} else {
		cmd, err = newCommand("git", "-C", opts.AbsLocalPath)
	}

	if err != nil {
		return nil, err
	}

	return &Git{
		Options: opts,
		command: cmd,
	}, nil
}

func (g *Git) Clone() error {
	if _, err := g.run("clone", g.RemoteURL, g.AbsLocalPath); err != nil {
		return errors.Wrap(err, "git clone failed")
	}
	return nil
}

func (g *Git) GetHEADCommitHash() (string, error) {
	res, err := g.run("rev-parse", "HEAD")
	if err != nil {
		return "", errors.Wrap(err, "git rev-parse HEAD failed")
	}
	return strings.TrimSpace(res), nil
}

// PullRebase pulls and rebases.
func (g *Git) PullRebase() error {
	_, err := g.run("pull", "--rebase")
	if err != nil {
		return errors.Wrap(err, "git pull failed")
	}
	return nil
}

func (g *Git) Add(files ...string) error {
	_, err := g.run(append([]string{"add"}, files...)...)
	return err
}

func (g *Git) Commit(commitMessage string) error {
	if err := g.checkWriteAllowed(); err != nil {
		return err
	}

	_, err := g.run(
		"-g", fmt.Sprintf(`user.name="%s"`, g.AuthorName),
		"-g", fmt.Sprintf(`user.email="%s"`, g.AuthorEmail),
		"commit",
		"--all",
		"--author", fmt.Sprintf(`"%s <%s>"`, g.AuthorName, g.AuthorEmail),
		"--message", fmt.Sprintf(`"%s"`, commitMessage),
	)
	if err != nil {
		return errors.Wrap(err, "git commit failed")
	}
	return nil
}

func (g *Git) Push() error {
	if err := g.checkWriteAllowed(); err != nil {
		return err
	}

	tmpPushURL, err := g.getPushURL()
	if err != nil {
		return err
	}

	if _, err := g.run("push", tmpPushURL, g.BranchName); err != nil {
		return errors.Wrap(err, "git push failed")
	}
	return nil
}

func (g *Git) GetRemoteURL() (string, error) {
	res, err := g.run("remote", "get-url", remoteName)
	if err != nil {
		return "", errors.Wrapf(err, "git remote get-url %s failed", remoteName)
	}
	return res, nil
}

func (g *Git) Fetch() error {
	if _, err := g.run("fetch", remoteName, g.BranchName); err != nil {
		return errors.Wrapf(err, "git fetch %s %s failed", remoteName, g.BranchName)
	}
	return nil
}

func (g *Git) getPushURL() (string, error) {
	remote, err := g.GetRemoteURL()
	if err != nil {
		return "", err
	}

	if g.GithubToken != "" {
		remote = strings.TrimPrefix(remote, "https://")
		return fmt.Sprintf("https://%s:%s@%s", g.AuthorName, g.GithubToken, remote), nil
	}

	// Use ssh
	return remote, nil
}

func (g *Git) checkWriteAllowed() error {
	// Write operations are only allowed with author name and email.
	if g.AuthorEmail == "" || g.AuthorName == "" {
		return errors.New("missing author name and/or email")
	}
	return nil
}
