//go:build windows

package daemon

import (
	"fmt"
	"os"
)

type stateLock struct {
	file *os.File
}

func acquireStateLock(stateDir string) (*stateLock, error) {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	return &stateLock{}, nil
}

func (l *stateLock) Release() error {
	return nil
}
