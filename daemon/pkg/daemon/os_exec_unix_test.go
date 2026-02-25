//go:build unix

package daemon

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestOSExecutorCancelKillsProcessGroup(t *testing.T) {
	exec := OSExecutor{}
	pidFile := t.TempDir() + "/child.pid"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan struct {
		code int
		err  error
	}, 1)
	go func() {
		code, err := exec.Run(ctx, CommandSpec{
			Raw:   "sleep 10 & echo $! > " + pidFile + "; while true; do sleep 1; done",
			Shell: true,
		})
		resultCh <- struct {
			code int
			err  error
		}{code: code, err: err}
	}()

	childPID := waitForChildPID(t, pidFile, time.Second)
	cancel()

	select {
	case res := <-resultCh:
		if res.err != nil {
			t.Fatalf("Run returned error: %v", res.err)
		}
		if res.code == 0 {
			t.Fatalf("expected non-zero exit code on cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for canceled Run to return")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processExists(childPID) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child process still alive after cancel: pid=%d", childPID)
}

func TestOSExecutorCancelEscalatesToKill(t *testing.T) {
	exec := OSExecutor{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan struct {
		code int
		err  error
	}, 1)
	go func() {
		code, err := exec.Run(ctx, CommandSpec{
			Raw:   "trap '' TERM; while true; do sleep 1; done",
			Shell: true,
		})
		resultCh <- struct {
			code int
			err  error
		}{code: code, err: err}
	}()

	time.Sleep(40 * time.Millisecond)
	cancel()

	select {
	case res := <-resultCh:
		if res.err != nil {
			t.Fatalf("Run returned error: %v", res.err)
		}
		if res.code == 0 {
			t.Fatalf("expected non-zero exit code on kill escalation")
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for kill escalation path")
	}
}

func waitForChildPID(t *testing.T, path string, timeout time.Duration) int {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
			if parseErr == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for child pid file %s", path)
	return 0
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.ESRCH) {
		return false
	}
	return true
}
