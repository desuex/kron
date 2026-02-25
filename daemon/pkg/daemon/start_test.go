package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStartOnce(t *testing.T) {
	cfg := writeTempDaemonConfig(t, "0 0 * * * name=backup command=true\n")
	err := Start(context.Background(), StartOptions{
		ConfigPath: cfg,
		Source:     "kron",
		StateDir:   t.TempDir(),
		Tick:       5 * time.Millisecond,
		Once:       true,
	})
	if err != nil {
		t.Fatalf("Start once error: %v", err)
	}
}

func TestStartReturnsOnCanceledContext(t *testing.T) {
	cfg := writeTempDaemonConfig(t, "0 0 * * * name=backup command=true\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Start(ctx, StartOptions{
		ConfigPath: cfg,
		Source:     "kron",
		StateDir:   t.TempDir(),
		Tick:       5 * time.Millisecond,
		Once:       false,
	})
	if err != nil {
		t.Fatalf("Start canceled context error: %v", err)
	}
}

func TestStartValidatesOptions(t *testing.T) {
	if err := Start(context.Background(), StartOptions{}); err == nil {
		t.Fatalf("expected missing config error")
	}
}

func TestStartOnceCronSource(t *testing.T) {
	cfg := writeTempDaemonConfig(t, "0 0 * * * root true\n")
	err := Start(context.Background(), StartOptions{
		ConfigPath: cfg,
		Source:     "cron",
		StateDir:   t.TempDir(),
		Tick:       5 * time.Millisecond,
		Once:       true,
	})
	if err != nil {
		t.Fatalf("Start once cron source error: %v", err)
	}
}

func TestStartRejectsUnknownSource(t *testing.T) {
	cfg := writeTempDaemonConfig(t, "0 0 * * * name=backup command=true\n")
	err := Start(context.Background(), StartOptions{
		ConfigPath: cfg,
		Source:     "bad",
		StateDir:   t.TempDir(),
		Once:       true,
	})
	if err == nil {
		t.Fatalf("expected unsupported source error")
	}
}

func TestStartAppliesDefaults(t *testing.T) {
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "jobs.kron")
	if err := os.WriteFile(cfg, []byte("0 0 * * * name=backup command=true\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	err = Start(context.Background(), StartOptions{
		ConfigPath: cfg,
		Source:     "",
		StateDir:   "",
		Tick:       0,
		Once:       true,
	})
	if err != nil {
		t.Fatalf("Start defaults error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmp, ".krond-state")); err != nil {
		t.Fatalf("expected default state dir to be created, got %v", err)
	}
}

func TestStartReturnsErrorWhenStateDirIsFile(t *testing.T) {
	tmp := t.TempDir()
	cfg := filepath.Join(tmp, "jobs.kron")
	if err := os.WriteFile(cfg, []byte("0 0 * * * name=backup command=true\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	stateFile := filepath.Join(tmp, "state-file")
	if err := os.WriteFile(stateFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	err := Start(context.Background(), StartOptions{
		ConfigPath: cfg,
		Source:     "kron",
		StateDir:   stateFile,
		Tick:       time.Millisecond,
		Once:       true,
	})
	if err == nil {
		t.Fatalf("expected error when state dir points to file")
	}
}

func TestStartForegroundReturnsStepError(t *testing.T) {
	cfg := writeTempDaemonConfig(t, "* * * * * name=boom command=/definitely/missing/command\n")
	err := Start(context.Background(), StartOptions{
		ConfigPath: cfg,
		Source:     "kron",
		StateDir:   t.TempDir(),
		Tick:       time.Millisecond,
		Once:       false,
	})
	if err == nil {
		t.Fatalf("expected start loop to return step error")
	}
}

func writeTempDaemonConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "daemon-*.kron")
	if err != nil {
		t.Fatalf("create temp config: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		t.Fatalf("write temp config: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp config: %v", err)
	}
	return f.Name()
}
