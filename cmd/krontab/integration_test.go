package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var integrationBinaryPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "krontab-integration-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	integrationBinaryPath = filepath.Join(tmpDir, "krontab-test")
	build := exec.Command("go", "build", "-o", integrationBinaryPath, ".")
	if out, err := build.CombinedOutput(); err != nil {
		panic("failed to build krontab binary for integration tests: " + err.Error() + "\n" + string(out))
	}

	os.Exit(m.Run())
}

func TestNextIntegrationText(t *testing.T) {
	cfg := writeTempKrontab(t, `
*/30 * * * * @win(after,0s) @dist(uniform) name=backup command=/usr/bin/backup
`)

	stdout, stderr, code, err := runKrontab("next", "backup",
		"--file", cfg,
		"--count", "2",
		"--at", "2026-02-24T10:07:00Z",
		"--format", "text",
	)
	if err != nil {
		t.Fatalf("runKrontab error: %v", err)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code: got %d stderr=%q", code, stderr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "period_start=2026-02-24T10:30:00Z") {
		t.Fatalf("missing first period in output: %q", stdout)
	}
	if !strings.Contains(stdout, "period_start=2026-02-24T11:00:00Z") {
		t.Fatalf("missing second period in output: %q", stdout)
	}
}

func TestNextIntegrationJSON(t *testing.T) {
	cfg := writeTempKrontab(t, `
*/30 * * * * @win(after,0s) @dist(uniform) name=backup command=/usr/bin/backup
`)

	stdout, stderr, code, err := runKrontab("next", "backup",
		"--file", cfg,
		"--count", "2",
		"--at", "2026-02-24T10:07:00Z",
		"--format", "json",
	)
	if err != nil {
		t.Fatalf("runKrontab error: %v", err)
	}
	if code != 0 {
		t.Fatalf("unexpected exit code: got %d stderr=%q", code, stderr)
	}

	var got nextResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode json output: %v", err)
	}
	if got.Job != "backup" {
		t.Fatalf("job mismatch: got %q want %q", got.Job, "backup")
	}
	if got.Count != 2 {
		t.Fatalf("count mismatch: got %d want %d", got.Count, 2)
	}
	if len(got.Decisions) != 2 {
		t.Fatalf("decisions length mismatch: got %d want %d", len(got.Decisions), 2)
	}
	if got.Decisions[0].PeriodStart.Format("2006-01-02T15:04:05Z07:00") != "2026-02-24T10:30:00Z" {
		t.Fatalf("first period mismatch: got %s", got.Decisions[0].PeriodStart)
	}
}

func TestNextIntegrationMissingJobExitCode(t *testing.T) {
	cfg := writeTempKrontab(t, `
0 0 * * * name=backup command=/usr/bin/backup
`)

	_, stderr, code, err := runKrontab("next", "missing",
		"--file", cfg,
		"--count", "1",
		"--at", "2026-02-24T10:07:00Z",
	)
	if err != nil {
		t.Fatalf("runKrontab error: %v", err)
	}
	if code != 1 {
		t.Fatalf("unexpected exit code: got %d want %d; stderr=%q", code, 1, stderr)
	}
	if !strings.Contains(stderr, "job not found") {
		t.Fatalf("expected job not found in stderr, got %q", stderr)
	}
}

func TestNextIntegrationInvalidCountExitCode(t *testing.T) {
	cfg := writeTempKrontab(t, `
0 0 * * * name=backup command=/usr/bin/backup
`)

	_, stderr, code, err := runKrontab("next", "backup",
		"--file", cfg,
		"--count", "0",
		"--at", "2026-02-24T10:07:00Z",
	)
	if err != nil {
		t.Fatalf("runKrontab error: %v", err)
	}
	if code != 2 {
		t.Fatalf("unexpected exit code: got %d want %d; stderr=%q", code, 2, stderr)
	}
	if !strings.Contains(stderr, "--count must be > 0") {
		t.Fatalf("expected count validation error in stderr, got %q", stderr)
	}
}

func runKrontab(args ...string) (stdout string, stderr string, exitCode int, err error) {
	cmd := exec.Command(integrationBinaryPath, args...)
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	if runErr == nil {
		return outBuf.String(), errBuf.String(), 0, nil
	}

	exitErr, ok := runErr.(*exec.ExitError)
	if !ok {
		return outBuf.String(), errBuf.String(), 0, runErr
	}
	return outBuf.String(), errBuf.String(), exitErr.ExitCode(), nil
}
