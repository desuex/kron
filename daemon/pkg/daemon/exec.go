package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Executor interface {
	Run(ctx context.Context, spec CommandSpec) (int, error)
}

type OSExecutor struct{}

func (OSExecutor) Run(ctx context.Context, spec CommandSpec) (int, error) {
	cmd, err := buildExecCommand(ctx, spec)
	if err != nil {
		return -1, err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil

	err = cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return -1, fmt.Errorf("execute command: %w", err)
}

func buildExecCommand(ctx context.Context, spec CommandSpec) (*exec.Cmd, error) {
	if strings.TrimSpace(spec.Raw) == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	var cmd *exec.Cmd
	if spec.Shell {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", spec.Raw)
	} else {
		parts := strings.Fields(spec.Raw)
		if len(parts) == 0 {
			return nil, fmt.Errorf("command cannot be empty")
		}
		cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
	}

	if spec.Cwd != "" {
		cmd.Dir = spec.Cwd
	}
	if len(spec.Env) > 0 {
		cmd.Env = append(os.Environ(), spec.Env...)
	}
	return cmd, nil
}
