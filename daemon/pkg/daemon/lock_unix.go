//go:build !windows

package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const stateLockFileName = ".krond.lock"

type stateLock struct {
	file     *os.File
	stateDir string
}

func acquireStateLock(stateDir string) (*stateLock, error) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}

	lockPath := filepath.Join(stateDir, stateLockFileName)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open state lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("state lock already held for %s", stateDir)
		}
		return nil, fmt.Errorf("acquire state lock: %w", err)
	}

	return &stateLock{file: f, stateDir: stateDir}, nil
}

func (l *stateLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}

	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	if unlockErr != nil || closeErr != nil {
		return errors.Join(unlockErr, closeErr)
	}
	return nil
}
