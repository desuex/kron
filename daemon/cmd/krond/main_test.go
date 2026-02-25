package main

import (
	"os"
	"syscall"
	"testing"
	"time"
)

func TestRunUsagePaths(t *testing.T) {
	if code := run([]string{"krond"}); code != 2 {
		t.Fatalf("expected exit 2 for missing args, got %d", code)
	}
	if code := run([]string{"krond", "help"}); code != 0 {
		t.Fatalf("expected exit 0 for help, got %d", code)
	}
	if code := run([]string{"krond", "start"}); code != 2 {
		t.Fatalf("expected exit 2 for missing config, got %d", code)
	}
}

func TestRunStartOnce(t *testing.T) {
	cfg := writeTempConfig(t, "0 0 * * * name=backup command=true\n")
	stateDir := t.TempDir()

	code := run([]string{
		"krond",
		"start",
		"--config", cfg,
		"--state-dir", stateDir,
		"--once",
	})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRunStartForegroundSignalExit(t *testing.T) {
	cfg := writeTempConfig(t, "0 0 * * * name=backup command=true\n")
	stateDir := t.TempDir()

	done := make(chan int, 1)
	go func() {
		code := run([]string{
			"krond",
			"start",
			"--config", cfg,
			"--state-dir", stateDir,
			"--tick", "10ms",
		})
		done <- code
	}()

	time.Sleep(30 * time.Millisecond)
	_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("expected clean shutdown code, got %d", code)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for krond shutdown")
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "krond-cmd-*.kron")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		t.Fatalf("write temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	return f.Name()
}
