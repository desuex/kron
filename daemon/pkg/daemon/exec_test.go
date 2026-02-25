package daemon

import (
	"context"
	"testing"
)

func TestBuildExecCommandDirect(t *testing.T) {
	cmd, err := buildExecCommand(context.Background(), CommandSpec{
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
	cmd, err := buildExecCommand(context.Background(), CommandSpec{
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
	if _, err := buildExecCommand(context.Background(), CommandSpec{}); err == nil {
		t.Fatalf("expected empty command error")
	}
}
