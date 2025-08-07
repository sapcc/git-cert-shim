// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/util/retry"
)

const remoteName = "origin"

type RepositorySyncer struct {
	logger     logr.Logger
	gitCli     *Git
	mtx        *sync.Mutex
	syncPeriod time.Duration
	syncSoon   chan struct{}
	hasSynced  bool
	dryRun     bool
}

func NewRepositorySyncerAndInit(logger logr.Logger, opts *Options, mtx *sync.Mutex) (*RepositorySyncer, error) {
	git, err := NewGit(opts)
	if err != nil {
		return nil, err
	}

	r := &RepositorySyncer{
		logger:     logger,
		gitCli:     git,
		mtx:        mtx,
		syncPeriod: opts.SyncPeriod,
		syncSoon:   make(chan struct{}, 1),
		hasSynced:  false,
		dryRun:     opts.DryRun,
	}

	start := time.Now()
	logger.Info("cloning repository. this might take a while..", "repository", opts.RemoteURL, "path", opts.AbsLocalPath)
	if err := r.clone(); err != nil {
		return nil, err
	}

	logger.Info("successfully initialized repository syncer", "path", opts.AbsLocalPath, "took", time.Since(start).String())
	return r, nil
}

func (r *RepositorySyncer) Start(ctx context.Context) error {
	defer close(r.syncSoon)

	go func() {
		ticker := time.NewTicker(r.syncPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-r.syncSoon:
				err := r.syncWithRetry()
				r.handleSyncError(err)
			case <-ticker.C:
				err := r.syncWithRetry()
				r.handleSyncError(err)
			case <-ctx.Done():
				return
			}
		}
	}()
	<-ctx.Done()
	return nil
}

func (r *RepositorySyncer) AddFilesAndCommit(commitMessage string, files ...string) error {
	res, err := r.gitCli.Status()
	if err != nil {
		return err
	}
	if res == "" {
		r.logger.V(1).Info("No changes to commit.")
		return nil
	}

	if err := r.gitCli.Add(files...); err != nil {
		return err
	}

	if err := r.gitCli.Commit(commitMessage); err != nil {
		return err
	}

	r.requireSync()
	return nil
}

func (r *RepositorySyncer) requireSync() {
	r.hasSynced = false
	r.syncSoon <- struct{}{}
}

func (r *RepositorySyncer) handleSyncError(err error) {
	if err != nil {
		fmt.Println("failed to sync", "err", err)
		r.requireSync()
		return
	}

	r.logger.Info("successfully synced")
	r.hasSynced = true
}

func (r *RepositorySyncer) clone() error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	err := r.gitCli.Clone()
	return err
}

func (r *RepositorySyncer) syncWithRetry() error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	err := retry.OnError(retry.DefaultBackoff,
		//nolint:gocritic
		func(err error) bool {
			// Retry the sync, if a git pull --rebase can help.
			return isErrFailedToPushSomeRefs(err)
		},
		func() error {
			remoteHeadCommitHash, err := r.gitCli.GetRemoteHEADCommitHash()
			if err != nil {
				return err
			}

			if err := r.gitCli.PullRebase(); err != nil {
				gitSyncErrorTotal.WithLabelValues("pull").Inc()
				return err
			}

			curHeadCommitHash, err := r.gitCli.GetHEADCommitHash()
			if err != nil {
				return err
			}

			r.logger.V(1).Info("Remote head commit hash, current head commit hash", remoteHeadCommitHash, curHeadCommitHash)

			// No changes. We're done.
			if remoteHeadCommitHash == curHeadCommitHash {
				return nil
			}

			if !r.dryRun {
				r.logger.V(1).Info("Pushing changes to repository.")
				if err := r.gitCli.Push(); err != nil {
					gitSyncErrorTotal.WithLabelValues("push").Inc()
					return err
				}
			}

			return nil
		})
	return err
}
