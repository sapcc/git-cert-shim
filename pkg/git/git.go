// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

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
	if err := opts.validate(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(opts.AbsLocalPath) {
		return nil, fmt.Errorf("requires an absolute path. cannot use: %s", opts.AbsLocalPath)
	}
	if err := util.EnsureDir(opts.AbsLocalPath, opts.IsEnsureEmptyDirectory); err != nil {
		return nil, errors.Wrapf(err, "failed to get or create path %s", opts.AbsLocalPath)
	}
	cmd, err := newCommand("git", "-C", opts.AbsLocalPath)
	if err != nil {
		return nil, err
	}
	return &Git{
		Options: opts,
		command: cmd,
	}, nil
}

func (g *Git) Clone() error {
	if res, err := g.run("clone",
		"--progress",
		"--depth", "1", "--single-branch", "--branch", g.BranchName,
		g.RemoteURL, g.AbsLocalPath,
	); err != nil {
		return errors.Wrapf(err, "git clone failed: %s", res)
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

func (g *Git) GetRemoteHEADCommitHash() (string, error) {
	res, err := g.run("ls-remote", "--heads", "-q")
	if err != nil {
		return "", errors.Wrap(err, "git ls-remote --heads -q failed")
	}
	return strings.TrimSpace(strings.Split(res, "\t")[0]), nil
}

// PullRebase pulls and rebases.
func (g *Git) PullRebase() error {
	_, err := g.run("rebase", "--abort")
	if err != nil {
		// ignore error if there is no rebase in progress
		if !strings.Contains(err.Error(), "no rebase in progress") {
			return errors.Wrap(err, "git rebase --abort failed")
		}
	}
	_, err = g.run(
		"-c", fmt.Sprintf(`user.name="%s"`, g.AuthorName),
		"-c", fmt.Sprintf(`user.email="%s"`, g.AuthorEmail),
		"pull",
		"--rebase",
	)
	if err != nil {
		return errors.Wrap(err, "git pull failed")
	}
	return nil
}

func (g *Git) Status() (string, error) {
	res, err := g.run("status", "-s")
	if err != nil {
		return "", errors.Wrap(err, "git status -s failed")
	}
	return strings.TrimSpace(res), nil
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
		"-c", fmt.Sprintf(`user.name="%s"`, g.AuthorName),
		"-c", fmt.Sprintf(`user.email="%s"`, g.AuthorEmail),
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
	return remote, nil
}

func (g *Git) checkWriteAllowed() error {
	// Write operations are only allowed with author name and email.
	if g.AuthorEmail == "" || g.AuthorName == "" {
		return errors.New("missing author name and/or email")
	}
	return nil
}
