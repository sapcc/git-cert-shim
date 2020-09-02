package git

import (
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
	mtx        sync.RWMutex
	syncPeriod time.Duration
	syncSoon   chan struct{}
	hasSynced  bool
}

func NewRepositorySyncerAndInit(logger logr.Logger, opts *Options) (*RepositorySyncer, error) {
	git, err := NewGit(opts)
	if err != nil {
		return nil, err
	}

	r := &RepositorySyncer{
		logger:     logger,
		gitCli:     git,
		mtx:        sync.RWMutex{},
		syncPeriod: opts.SyncPeriod,
		syncSoon:   make(chan struct{}, 1),
		hasSynced:  false,
	}

	if err := r.clone(); err != nil {
		return nil, err
	}

	logger.Info("successfully initialized repository syncer", "path", opts.AbsLocalPath)
	return r, nil
}

func (r *RepositorySyncer) Start(stop <-chan struct{}) error {
	defer close(r.syncSoon)

	ticker := time.NewTicker(r.syncPeriod)
	go func() {
		for {
			select {
			case <-r.syncSoon:
				err := r.syncWithRetry()
				r.handleSyncError(err)
			case <-ticker.C:
				err := r.syncWithRetry()
				r.handleSyncError(err)
			case <-stop:
				ticker.Stop()
			}
		}
	}()

	<-stop
	return nil
}

func (r *RepositorySyncer) AddFilesAndCommit(commitMessage string, files ...string) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()

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
		func(err error) bool {
			// Retry the sync, if a git pull --rebase can help.
			return isErrFailedToPushSomeRefs(err)
		},
		func() error {
			oldHeadCommitHash, err := r.gitCli.GetHEADCommitHash()
			if err != nil {
				return err
			}

			if err := r.gitCli.Fetch(); err != nil {
				return err
			}

			if err := r.gitCli.PullRebase(); err != nil {
				return err
			}

			curHeadCommitHash, err := r.gitCli.GetHEADCommitHash()
			if err != nil {
				return err
			}

			// No changes. We're done.
			if oldHeadCommitHash == curHeadCommitHash {
				return nil
			}

			err = r.gitCli.Push()
			return err
		})

	return err
}
