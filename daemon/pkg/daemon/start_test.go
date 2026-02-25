package daemon

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestStartOnce(t *testing.T) {
	cfg := writeTempDaemonConfig(t, "0 0 * * * name=backup command=true\n")
	err := Start(context.Background(), StartOptions{
		ConfigPath: cfg,
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
