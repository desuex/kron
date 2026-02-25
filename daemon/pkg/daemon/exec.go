package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Executor interface {
	Run(ctx context.Context, spec CommandSpec) (int, error)
}

type OSExecutor struct{}

const processStopGrace = 500 * time.Millisecond

type userLookupFunc func(raw string) (int, int, error)
type groupLookupFunc func(raw string) (int, error)

func (OSExecutor) Run(ctx context.Context, spec CommandSpec) (int, error) {
	cmd, err := buildExecCommand(spec)
	if err != nil {
		return -1, err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil

	if err := applyExecSecurity(cmd, spec); err != nil {
		return -1, err
	}

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

func applyExecSecurity(cmd *exec.Cmd, spec CommandSpec) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	cred, err := resolveCredential(spec)
	if err != nil {
		return fmt.Errorf("configure execution identity: %w", err)
	}
	if cred != nil {
		cmd.SysProcAttr.Credential = cred
	}
	return nil
}

func resolveCredential(spec CommandSpec) (*syscall.Credential, error) {
	return resolveCredentialWithContext(spec, os.Geteuid(), os.Getegid(), lookupUserIDGID, lookupGroupID)
}

func resolveCredentialWithContext(spec CommandSpec, currentUID, currentGID int, lookupUser userLookupFunc, lookupGroup groupLookupFunc) (*syscall.Credential, error) {
	userSpec := strings.TrimSpace(spec.User)
	groupSpec := strings.TrimSpace(spec.Group)
	if userSpec == "" && groupSpec == "" {
		return nil, nil
	}

	targetUID := currentUID
	targetGID := currentGID

	if userSpec != "" {
		uid, gid, err := lookupUser(userSpec)
		if err != nil {
			return nil, err
		}
		targetUID = uid
		if groupSpec == "" {
			targetGID = gid
		}
	}
	if groupSpec != "" {
		gid, err := lookupGroup(groupSpec)
		if err != nil {
			return nil, err
		}
		targetGID = gid
	}

	if currentUID != 0 {
		if userSpec != "" && targetUID != currentUID {
			return nil, fmt.Errorf("user switching requires root (current uid=%d, target uid=%d)", currentUID, targetUID)
		}
		if groupSpec != "" && targetGID != currentGID {
			return nil, fmt.Errorf("group switching requires root (current gid=%d, target gid=%d)", currentGID, targetGID)
		}
		return nil, nil
	}

	return &syscall.Credential{
		Uid: uint32(targetUID),
		Gid: uint32(targetGID),
	}, nil
}

func lookupUserIDGID(raw string) (int, int, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return 0, 0, fmt.Errorf("user cannot be empty")
	}

	var (
		u   *user.User
		err error
	)
	if _, convErr := strconv.Atoi(name); convErr == nil {
		u, err = user.LookupId(name)
	} else {
		u, err = user.Lookup(name)
	}
	if err != nil {
		return 0, 0, fmt.Errorf("lookup user %q: %w", raw, err)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, fmt.Errorf("parse uid for user %q: %w", raw, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, 0, fmt.Errorf("parse gid for user %q: %w", raw, err)
	}
	return uid, gid, nil
}

func lookupGroupID(raw string) (int, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return 0, fmt.Errorf("group cannot be empty")
	}

	var (
		g   *user.Group
		err error
	)
	if _, convErr := strconv.Atoi(name); convErr == nil {
		g, err = user.LookupGroupId(name)
	} else {
		g, err = user.LookupGroup(name)
	}
	if err != nil {
		return 0, fmt.Errorf("lookup group %q: %w", raw, err)
	}

	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return 0, fmt.Errorf("parse gid for group %q: %w", raw, err)
	}
	return gid, nil
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
