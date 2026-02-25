package daemon

import (
	"context"
	"os"
	"os/user"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOSExecutorRunSuccessAndExitCode(t *testing.T) {
	exec := OSExecutor{}

	code, err := exec.Run(context.Background(), CommandSpec{Raw: "exit 0", Shell: true})
	if err != nil {
		t.Fatalf("Run success error: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	code, err = exec.Run(context.Background(), CommandSpec{Raw: "exit 7", Shell: true})
	if err != nil {
		t.Fatalf("Run shell non-zero error: %v", err)
	}
	if code != 7 {
		t.Fatalf("expected exit code 7, got %d", code)
	}
}

func TestOSExecutorRunInvalidCommand(t *testing.T) {
	exec := OSExecutor{}
	_, err := exec.Run(context.Background(), CommandSpec{Raw: "/definitely/missing/command"})
	if err == nil {
		t.Fatalf("expected invalid command error")
	}
}

func TestOSExecutorRunRejectsEmptyCommand(t *testing.T) {
	exec := OSExecutor{}
	_, err := exec.Run(context.Background(), CommandSpec{Raw: "   ", Shell: false})
	if err == nil {
		t.Fatalf("expected empty command error")
	}
}

func TestOSExecutorRunTimeoutContext(t *testing.T) {
	exec := OSExecutor{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	code, err := exec.Run(ctx, CommandSpec{Raw: "sleep 1", Shell: true})
	if err != nil {
		t.Fatalf("Run timeout returned unexpected error: %v", err)
	}
	if code == 0 {
		t.Fatalf("expected non-zero exit when command is canceled by timeout")
	}
}

func TestOSExecutorRunSameUserAllowed(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Fatalf("current user error: %v", err)
	}

	exec := OSExecutor{}
	code, err := exec.Run(context.Background(), CommandSpec{
		Raw:   "exit 0",
		Shell: true,
		User:  current.Username,
	})
	if err != nil {
		t.Fatalf("Run same user error: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestOSExecutorRejectsDifferentUserWithoutRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("non-root behavior test")
	}

	exec := OSExecutor{}
	_, err := exec.Run(context.Background(), CommandSpec{
		Raw:   "exit 0",
		Shell: true,
		User:  "root",
	})
	if err == nil {
		t.Fatalf("expected root requirement error")
	}
	if !strings.Contains(err.Error(), "requires root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOSExecutorRejectsUnknownUserOrGroup(t *testing.T) {
	exec := OSExecutor{}

	_, err := exec.Run(context.Background(), CommandSpec{
		Raw:   "exit 0",
		Shell: true,
		User:  "__definitely_missing_user__",
	})
	if err == nil || !strings.Contains(err.Error(), "lookup user") {
		t.Fatalf("expected lookup user error, got %v", err)
	}

	_, err = exec.Run(context.Background(), CommandSpec{
		Raw:   "exit 0",
		Shell: true,
		Group: "__definitely_missing_group__",
	})
	if err == nil || !strings.Contains(err.Error(), "lookup group") {
		t.Fatalf("expected lookup group error, got %v", err)
	}
}

func TestOSExecutorRunSameGroupAllowed(t *testing.T) {
	exec := OSExecutor{}
	code, err := exec.Run(context.Background(), CommandSpec{
		Raw:   "exit 0",
		Shell: true,
		Group: strconv.Itoa(os.Getegid()),
	})
	if err != nil {
		t.Fatalf("Run same group error: %v", err)
	}
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}
