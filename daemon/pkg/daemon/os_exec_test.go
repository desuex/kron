package daemon

import (
	"context"
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
