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
	if code := run([]string{"krond", "-h"}); code != 0 {
		t.Fatalf("expected exit 0 for -h, got %d", code)
	}
	if code := run([]string{"krond", "--help"}); code != 0 {
		t.Fatalf("expected exit 0 for --help, got %d", code)
	}
	if code := run([]string{"krond", "unknown"}); code != 2 {
		t.Fatalf("expected exit 2 for unknown command, got %d", code)
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

func TestRunStartInvalidSource(t *testing.T) {
	cfg := writeTempConfig(t, "0 0 * * * name=backup command=true\n")
	stateDir := t.TempDir()

	code := run([]string{
		"krond",
		"start",
		"--config", cfg,
		"--source", "invalid",
		"--state-dir", stateDir,
		"--once",
	})
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
}

func TestRunStartCronSourceOnce(t *testing.T) {
	cfg := writeTempConfig(t, "0 0 * * * root true\n")
	stateDir := t.TempDir()

	code := run([]string{
		"krond",
		"start",
		"--config", cfg,
		"--source", "cron",
		"--state-dir", stateDir,
		"--once",
	})
	if code != 0 {
		t.Fatalf("expected exit 0 for cron source, got %d", code)
	}
}

func TestRunStartFlagParsingErrors(t *testing.T) {
	cfg := writeTempConfig(t, "0 0 * * * name=backup command=true\n")
	stateDir := t.TempDir()

	code := run([]string{
		"krond",
		"start",
		"--config", cfg,
		"--state-dir", stateDir,
		"--tick", "not-a-duration",
		"--once",
	})
	if code != 2 {
		t.Fatalf("expected exit 2 for bad duration flag, got %d", code)
	}

	code = run([]string{
		"krond",
		"start",
		"--config", cfg,
		"--state-dir", stateDir,
		"--once",
		"extra-positional",
	})
	if code != 2 {
		t.Fatalf("expected exit 2 for positional args, got %d", code)
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
