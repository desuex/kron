package daemon

import (
	"fmt"
	"os/exec"
	"syscall"
	"testing"
)

func TestBuildExecCommandDirect(t *testing.T) {
	cmd, err := buildExecCommand(CommandSpec{
		Raw:   "/bin/echo hello world",
		Shell: false,
		Cwd:   "/tmp",
		Env:   []string{"FOO=bar"},
	})
	if err != nil {
		t.Fatalf("buildExecCommand error: %v", err)
	}
	if cmd.Path != "/bin/echo" {
		t.Fatalf("path mismatch: %q", cmd.Path)
	}
	if len(cmd.Args) != 3 || cmd.Args[1] != "hello" || cmd.Args[2] != "world" {
		t.Fatalf("args mismatch: %+v", cmd.Args)
	}
	if cmd.Dir != "/tmp" {
		t.Fatalf("dir mismatch: %q", cmd.Dir)
	}
}

func TestBuildExecCommandShell(t *testing.T) {
	cmd, err := buildExecCommand(CommandSpec{
		Raw:   "echo hello",
		Shell: true,
	})
	if err != nil {
		t.Fatalf("buildExecCommand shell error: %v", err)
	}
	if cmd.Path != "/bin/sh" {
		t.Fatalf("path mismatch: %q", cmd.Path)
	}
	if len(cmd.Args) != 3 || cmd.Args[1] != "-c" || cmd.Args[2] != "echo hello" {
		t.Fatalf("args mismatch: %+v", cmd.Args)
	}
}

func TestBuildExecCommandRejectsEmpty(t *testing.T) {
	if _, err := buildExecCommand(CommandSpec{}); err == nil {
		t.Fatalf("expected empty command error")
	}
}

func TestProcessGroupHelpersHandleNilAndExitedProcess(t *testing.T) {
	cmd := &exec.Cmd{}
	terminateProcessGroup(cmd)
	killProcessGroup(cmd)

	if err := signalProcessGroup(cmd, syscall.SIGTERM); err != nil {
		t.Fatalf("expected nil error for nil process, got %v", err)
	}

	doneCmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := doneCmd.Start(); err != nil {
		t.Fatalf("start command: %v", err)
	}
	if err := doneCmd.Wait(); err != nil {
		t.Fatalf("wait command: %v", err)
	}
	if err := signalProcessGroup(doneCmd, syscall.SIGTERM); err != nil {
		t.Fatalf("expected nil error for exited process, got %v", err)
	}
}

func TestExitCodeFromWaitUnknownError(t *testing.T) {
	if got := exitCodeFromWait(fmt.Errorf("unknown")); got != -1 {
		t.Fatalf("expected -1, got %d", got)
	}
}
