package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
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

func TestBuildExecCommandRejectsWhitespaceDirect(t *testing.T) {
	if _, err := buildExecCommand(CommandSpec{Raw: "   ", Shell: false}); err == nil {
		t.Fatalf("expected empty command error for whitespace input")
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

func TestResolveCredentialWithContextNone(t *testing.T) {
	cred, err := resolveCredentialWithContext(CommandSpec{}, 1000, 1000,
		func(string) (int, int, error) {
			t.Fatalf("lookup user should not be called")
			return 0, 0, nil
		},
		func(string) (int, error) {
			t.Fatalf("lookup group should not be called")
			return 0, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred != nil {
		t.Fatalf("expected nil credential, got %+v", cred)
	}
}

func TestResolveCredentialWithContextRootUserAndGroup(t *testing.T) {
	cred, err := resolveCredentialWithContext(
		CommandSpec{User: "svc", Group: "ops"},
		0,
		0,
		func(raw string) (int, int, error) {
			if raw != "svc" {
				t.Fatalf("unexpected user lookup arg: %q", raw)
			}
			return 1234, 2000, nil
		},
		func(raw string) (int, error) {
			if raw != "ops" {
				t.Fatalf("unexpected group lookup arg: %q", raw)
			}
			return 3000, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred == nil {
		t.Fatalf("expected credential")
	}
	if cred.Uid != 1234 || cred.Gid != 3000 {
		t.Fatalf("credential mismatch: %+v", cred)
	}
}

func TestResolveCredentialWithContextRootGroupOnly(t *testing.T) {
	cred, err := resolveCredentialWithContext(
		CommandSpec{Group: "ops"},
		0,
		44,
		func(string) (int, int, error) {
			t.Fatalf("lookup user should not be called")
			return 0, 0, nil
		},
		func(raw string) (int, error) {
			if raw != "ops" {
				t.Fatalf("unexpected group lookup arg: %q", raw)
			}
			return 777, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred == nil {
		t.Fatalf("expected credential")
	}
	if cred.Uid != 0 || cred.Gid != 777 {
		t.Fatalf("credential mismatch: %+v", cred)
	}
}

func TestResolveCredentialWithContextLookupErrors(t *testing.T) {
	userErr := errors.New("user failed")
	_, err := resolveCredentialWithContext(
		CommandSpec{User: "svc"},
		0,
		0,
		func(string) (int, int, error) { return 0, 0, userErr },
		func(string) (int, error) { return 0, nil },
	)
	if !errors.Is(err, userErr) {
		t.Fatalf("expected user lookup error, got %v", err)
	}

	groupErr := errors.New("group failed")
	_, err = resolveCredentialWithContext(
		CommandSpec{Group: "ops"},
		0,
		0,
		func(string) (int, int, error) { return 0, 0, nil },
		func(string) (int, error) { return 0, groupErr },
	)
	if !errors.Is(err, groupErr) {
		t.Fatalf("expected group lookup error, got %v", err)
	}
}

func TestResolveCredentialWithContextNonRootRejectsSwitch(t *testing.T) {
	_, err := resolveCredentialWithContext(
		CommandSpec{User: "other"},
		1000,
		1000,
		func(string) (int, int, error) { return 2000, 2000, nil },
		func(string) (int, error) { return 0, nil },
	)
	if err == nil {
		t.Fatalf("expected non-root user switch rejection")
	}

	_, err = resolveCredentialWithContext(
		CommandSpec{Group: "other"},
		1000,
		1000,
		func(string) (int, int, error) { return 0, 0, nil },
		func(string) (int, error) { return 2000, nil },
	)
	if err == nil {
		t.Fatalf("expected non-root group switch rejection")
	}
}

func TestResolveCredentialWithContextNonRootSameIdentityNoCredential(t *testing.T) {
	cred, err := resolveCredentialWithContext(
		CommandSpec{User: "current", Group: "current"},
		1000,
		1001,
		func(string) (int, int, error) { return 1000, 1001, nil },
		func(string) (int, error) { return 1001, nil },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred != nil {
		t.Fatalf("expected nil credential for non-root same identity, got %+v", cred)
	}
}

func TestApplyExecSecuritySetsProcessGroup(t *testing.T) {
	cmd := exec.Command("/bin/echo", "ok")
	if err := applyExecSecurity(cmd, CommandSpec{}); err != nil {
		t.Fatalf("applyExecSecurity error: %v", err)
	}
	if cmd.SysProcAttr == nil {
		t.Fatalf("expected SysProcAttr to be set")
	}
	if !cmd.SysProcAttr.Setpgid {
		t.Fatalf("expected Setpgid=true")
	}
	if cmd.SysProcAttr.Credential != nil {
		t.Fatalf("expected no credential for empty user/group")
	}
}

func TestApplyExecSecurityRejectsInvalidIdentity(t *testing.T) {
	cmd := exec.Command("/bin/echo", "ok")
	err := applyExecSecurity(cmd, CommandSpec{User: "__definitely_missing_user__"})
	if err == nil {
		t.Fatalf("expected identity error")
	}
	if !strings.Contains(err.Error(), "configure execution identity") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLookupUserIDGIDValidationAndLookup(t *testing.T) {
	if _, _, err := lookupUserIDGID("   "); err == nil {
		t.Fatalf("expected empty user validation error")
	}

	current, err := user.Current()
	if err != nil {
		t.Fatalf("current user error: %v", err)
	}

	wantUID, err := strconv.Atoi(current.Uid)
	if err != nil {
		t.Fatalf("parse current uid: %v", err)
	}
	wantGID, err := strconv.Atoi(current.Gid)
	if err != nil {
		t.Fatalf("parse current gid: %v", err)
	}

	gotUID, gotGID, err := lookupUserIDGID(current.Username)
	if err != nil {
		t.Fatalf("lookup by username error: %v", err)
	}
	if gotUID != wantUID || gotGID != wantGID {
		t.Fatalf("lookup by username mismatch: got uid/gid=%d/%d want %d/%d", gotUID, gotGID, wantUID, wantGID)
	}

	gotUID, gotGID, err = lookupUserIDGID(current.Uid)
	if err != nil {
		t.Fatalf("lookup by uid error: %v", err)
	}
	if gotUID != wantUID || gotGID != wantGID {
		t.Fatalf("lookup by uid mismatch: got uid/gid=%d/%d want %d/%d", gotUID, gotGID, wantUID, wantGID)
	}
}

func TestLookupGroupIDValidationAndLookup(t *testing.T) {
	if _, err := lookupGroupID(" "); err == nil {
		t.Fatalf("expected empty group validation error")
	}

	current, err := user.Current()
	if err != nil {
		t.Fatalf("current user error: %v", err)
	}

	wantGID, err := strconv.Atoi(current.Gid)
	if err != nil {
		t.Fatalf("parse current gid: %v", err)
	}

	gotGID, err := lookupGroupID(current.Gid)
	if err != nil {
		t.Fatalf("lookup by gid error: %v", err)
	}
	if gotGID != wantGID {
		t.Fatalf("lookup by gid mismatch: got %d want %d", gotGID, wantGID)
	}
}

func TestSignalProcessGroupUnknownPID(t *testing.T) {
	cmd := &exec.Cmd{Process: &os.Process{Pid: 999999}}
	_ = signalProcessGroup(cmd, syscall.SIGTERM)
}
