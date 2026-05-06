//go:build unix

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRuntimeRestartDoesNotDuplicateActivePeriod(t *testing.T) {
	cfg := mustJobConfig(t)
	tmp := t.TempDir()
	countPath := filepath.Join(tmp, "runs.log")
	cfg.Command = CommandSpec{
		Raw:   "echo run >> " + countPath + "; trap 'exit 0' TERM INT; while true; do sleep 1; done",
		Shell: true,
	}

	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := FileStateStore{Dir: filepath.Join(tmp, "state")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt1, err := newRuntime([]JobConfig{cfg}, store, OSExecutor{}, now)
	if err != nil {
		t.Fatalf("newRuntime rt1 error: %v", err)
	}
	if err := rt1.Step(ctx, now); err != nil {
		t.Fatalf("rt1 Step error: %v", err)
	}

	waitForFileContent(t, countPath, "run")

	rt2, err := newRuntime([]JobConfig{cfg}, store, OSExecutor{}, now)
	if err != nil {
		t.Fatalf("newRuntime rt2 error: %v", err)
	}
	if err := rt2.Step(context.Background(), now); err != nil {
		t.Fatalf("rt2 Step error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	runs := readLinesTrimmed(t, countPath)
	if len(runs) != 1 {
		t.Fatalf("expected exactly one run after restart recovery, got %v", runs)
	}

	cancel()
	waitRuntimeIdle(t, rt1)
}

func waitForFileContent(t *testing.T, path, wantSubstring string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(raw), wantSubstring) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s to contain %q", path, wantSubstring)
}

func readLinesTrimmed(t *testing.T, path string) []string {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
