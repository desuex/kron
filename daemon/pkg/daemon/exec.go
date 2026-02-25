package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type Executor interface {
	Run(ctx context.Context, spec CommandSpec) (int, error)
}

type OSExecutor struct{}

const processStopGrace = 500 * time.Millisecond

func (OSExecutor) Run(ctx context.Context, spec CommandSpec) (int, error) {
	cmd, err := buildExecCommand(spec)
	if err != nil {
		return -1, err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return -1, fmt.Errorf("execute command: %w", err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		return exitCodeFromWait(err), nil
	case <-ctx.Done():
		terminateProcessGroup(cmd)
		select {
		case err := <-waitCh:
			return exitCodeFromWait(err), nil
		case <-time.After(processStopGrace):
			killProcessGroup(cmd)
			err := <-waitCh
			return exitCodeFromWait(err), nil
		}
	}
}

func buildExecCommand(spec CommandSpec) (*exec.Cmd, error) {
	if strings.TrimSpace(spec.Raw) == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	var cmd *exec.Cmd
	if spec.Shell {
		cmd = exec.Command("/bin/sh", "-c", spec.Raw)
	} else {
		parts := strings.Fields(spec.Raw)
		if len(parts) == 0 {
			return nil, fmt.Errorf("command cannot be empty")
		}
		cmd = exec.Command(parts[0], parts[1:]...)
	}

	if spec.Cwd != "" {
		cmd.Dir = spec.Cwd
	}
	if len(spec.Env) > 0 {
		cmd.Env = append(os.Environ(), spec.Env...)
	}
	return cmd, nil
}

func terminateProcessGroup(cmd *exec.Cmd) {
	_ = signalProcessGroup(cmd, syscall.SIGTERM)
}

func killProcessGroup(cmd *exec.Cmd) {
	_ = signalProcessGroup(cmd, syscall.SIGKILL)
}

func signalProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil && pgid > 0 {
		if err := syscall.Kill(-pgid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
		return nil
	}

	err = cmd.Process.Signal(sig)
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func exitCodeFromWait(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
