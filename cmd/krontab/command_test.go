package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	argFile                = "--file"
	argFormat              = "--format"
	argCount               = "--count"
	at20260224T1000        = "2026-02-24T10:00:00Z"
	at20260224T1007        = "2026-02-24T10:07:00Z"
	errExitCodeMismatch    = "exit code mismatch: got %d want %d"
	errExitCodeMismatchErr = "exit code mismatch: got %d want %d stderr=%q"
	errJSONParse           = "json parse error: %v"
	errRunExplain          = "runExplain error: %v"
	errRunNext             = "runNext error: %v"
	errStderrMismatch      = "stderr mismatch: got %q"
)

func TestRunDispatchBasic(t *testing.T) {
	var code int
	stdout, _ := captureOutput(t, func() {
		code = run([]string{"krontab"})
	})
	if code != 2 {
		t.Fatalf(errExitCodeMismatch, code, 2)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Fatalf("expected usage output, got %q", stdout)
	}

	stdout, _ = captureOutput(t, func() {
		code = run([]string{"krontab", "help"})
	})
	if code != 0 {
		t.Fatalf(errExitCodeMismatch, code, 0)
	}
	if !strings.Contains(stdout, "krontab next") {
		t.Fatalf("expected next command in usage, got %q", stdout)
	}

	stdout, _ = captureOutput(t, func() {
		code = run([]string{"krontab", "--help"})
	})
	if code != 0 {
		t.Fatalf(errExitCodeMismatch, code, 0)
	}

	stdout, _ = captureOutput(t, func() {
		code = run([]string{"krontab", "-h"})
	})
	if code != 0 {
		t.Fatalf(errExitCodeMismatch, code, 0)
	}

	stdout, _ = captureOutput(t, func() {
		code = run([]string{"krontab", "unknown"})
	})
	if code != 2 {
		t.Fatalf(errExitCodeMismatch, code, 2)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Fatalf("expected usage output, got %q", stdout)
	}
}

func TestRunLintEndToEnd(t *testing.T) {
	valid := writeTempKrontab(t, `0 0 * * * name=backup command=/usr/bin/backup`)
	invalid := writeTempKrontab(t, `0 0 * * * name=Bad_Name command=`)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = run([]string{"krontab", "lint", argFile, valid, argFormat, "text"})
	})
	if code != 0 {
		t.Fatalf(errExitCodeMismatchErr, code, 0, stderr)
	}
	if !strings.Contains(stdout, "OK:") {
		t.Fatalf("expected OK output, got %q", stdout)
	}

	stdout, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "lint", argFile, invalid, argFormat, "text"})
	})
	if code != 1 {
		t.Fatalf(errExitCodeMismatchErr, code, 1, stderr)
	}
	if !strings.Contains(stdout, "INVALID:") {
		t.Fatalf("expected INVALID output, got %q", stdout)
	}

	stdout, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "lint", argFile, valid, argFormat, "json"})
	})
	if code != 0 {
		t.Fatalf(errExitCodeMismatchErr, code, 0, stderr)
	}
	var parsed lintResult
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf(errJSONParse, err)
	}
	if !parsed.Valid {
		t.Fatalf("expected valid lint result: %+v", parsed)
	}

	_, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "lint"})
	})
	if code != 2 {
		t.Fatalf(errExitCodeMismatchErr, code, 2, stderr)
	}

	_, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "lint", argFile, valid, argFormat, "bad"})
	})
	if code != 2 {
		t.Fatalf(errExitCodeMismatchErr, code, 2, stderr)
	}

	_, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "lint", argFile, "/tmp/does-not-exist.kron"})
	})
	if code != 2 {
		t.Fatalf(errExitCodeMismatchErr, code, 2, stderr)
	}

	_, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "lint", argFile, valid, "extra"})
	})
	if code != 2 {
		t.Fatalf(errExitCodeMismatchErr, code, 2, stderr)
	}

	_, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "lint", "--bad-flag"})
	})
	if code != 2 {
		t.Fatalf(errExitCodeMismatchErr, code, 2, stderr)
	}
}

func TestRunExplainPaths(t *testing.T) { // NOSONAR
	cfg := writeTempKrontab(t, `
0 0 * * * @win(around,30m) @dist(uniform) name=backup command=/usr/bin/backup
`)

	stdout, _ := captureOutput(t, func() {
		err := runExplain([]string{"backup", argFile, cfg, "--at", at20260224T1000, argFormat, "text"})
		if err != nil {
			t.Fatalf(errRunExplain, err)
		}
	})
	if !strings.Contains(stdout, "chosen_time:") {
		t.Fatalf("expected chosen_time in output, got %q", stdout)
	}

	stdout, _ = captureOutput(t, func() {
		err := runExplain([]string{"backup", argFile, cfg, "--at", at20260224T1000, argFormat, "json"})
		if err != nil {
			t.Fatalf(errRunExplain, err)
		}
	})
	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf(errJSONParse, err)
	}
	if parsed["job"] != "backup" {
		t.Fatalf("unexpected job in json: %#v", parsed["job"])
	}

	err := runExplain([]string{"backup", argFile, cfg})
	if err == nil || !strings.Contains(err.Error(), "--at is required") {
		t.Fatalf("expected required-at error, got %v", err)
	}

	err = runExplain([]string{"backup", "--at", "not-a-time"})
	if err == nil || !strings.Contains(err.Error(), "invalid --at value") {
		t.Fatalf("expected invalid-at error, got %v", err)
	}

	err = runExplain([]string{"backup", "--at", at20260224T1000, "--mode", "bad"})
	if err == nil || !strings.Contains(err.Error(), "invalid window mode") {
		t.Fatalf("expected invalid mode error, got %v", err)
	}

	stdout, _ = captureOutput(t, func() {
		err := runExplain([]string{"backup", "--at", at20260224T1000, "--window", "30m", "--dist", "skewLate"})
		if err != nil {
			t.Fatalf(errRunExplain, err)
		}
	})
	if !strings.Contains(stdout, "distribution: skewLate") {
		t.Fatalf("expected skewLate in output, got %q", stdout)
	}

	err = runExplain([]string{"missing", argFile, cfg, "--at", at20260224T1000})
	if !errors.Is(err, errJobNotFound) {
		t.Fatalf("expected errJobNotFound, got %v", err)
	}

	err = runExplain([]string{"backup", argFile, cfg, "--at", at20260224T1000, argFormat, "bad"})
	if err == nil || !strings.Contains(err.Error(), "invalid --format value") {
		t.Fatalf("expected invalid format error, got %v", err)
	}
}

func TestRunExplainAppliesTimezoneAndSeedFromConfig(t *testing.T) {
	cfg := writeTempKrontab(t, `
0 9 * * * @tz(America/New_York) @win(after,0s) @seed(daily,salt=nyc) @only(hours=9;dow=TUE) name=backup command=/usr/bin/backup
`)

	stdout, _ := captureOutput(t, func() {
		err := runExplain([]string{"backup", argFile, cfg, "--at", "2026-02-24T14:00:00Z", argFormat, "json"})
		if err != nil {
			t.Fatalf(errRunExplain, err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf(errJSONParse, err)
	}
	if parsed["chosenTime"] != "2026-02-24T14:00:00Z" {
		t.Fatalf("chosenTime mismatch: got %#v", parsed["chosenTime"])
	}
	if parsed["seedStrategy"] != "daily" {
		t.Fatalf("seedStrategy mismatch: got %#v", parsed["seedStrategy"])
	}
	if unsched, ok := parsed["unschedulable"].(bool); !ok || unsched {
		t.Fatalf("expected schedulable decision, got unschedulable=%#v", parsed["unschedulable"])
	}
}

func TestRunExplainAppliesSkewShapeFromConfig(t *testing.T) {
	defaultCfg := writeTempKrontab(t, `
0 9 * * * @tz(UTC) @win(around,90m) @dist(skewLate) name=backup command=/usr/bin/backup
`)
	shapedCfg := writeTempKrontab(t, `
0 9 * * * @tz(UTC) @win(around,90m) @dist(skewLate,shape=4) name=backup command=/usr/bin/backup
`)

	parseChosen := func(cfg string) time.Time {
		stdout, _ := captureOutput(t, func() {
			err := runExplain([]string{"backup", argFile, cfg, "--at", "2026-03-01T09:00:00Z", argFormat, "json"})
			if err != nil {
				t.Fatalf(errRunExplain, err)
			}
		})

		var parsed map[string]any
		if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
			t.Fatalf(errJSONParse, err)
		}
		chosenRaw, ok := parsed["chosenTime"].(string)
		if !ok || chosenRaw == "" {
			t.Fatalf("missing chosenTime in parsed json: %#v", parsed["chosenTime"])
		}
		chosen, err := time.Parse(time.RFC3339, chosenRaw)
		if err != nil {
			t.Fatalf("parse chosenTime: %v", err)
		}
		return chosen
	}

	defaultChosen := parseChosen(defaultCfg)
	shapedChosen := parseChosen(shapedCfg)

	if !shapedChosen.After(defaultChosen) {
		t.Fatalf("expected shape=4 skewLate to choose later time: default=%s shaped=%s", defaultChosen, shapedChosen)
	}
}

func TestRunNextPaths(t *testing.T) { // NOSONAR
	cfg := writeTempKrontab(t, `
*/30 * * * * @win(after,0s) @dist(uniform) name=backup command=/usr/bin/backup
`)

	stdout, _ := captureOutput(t, func() {
		err := runNext([]string{"backup", argFile, cfg, argCount, "2", "--at", at20260224T1007, argFormat, "text"})
		if err != nil {
			t.Fatalf(errRunNext, err)
		}
	})
	if !strings.Contains(stdout, "1. period_start=") {
		t.Fatalf("expected numbered decisions in output, got %q", stdout)
	}

	stdout, _ = captureOutput(t, func() {
		err := runNext([]string{"backup", argFile, cfg, argCount, "2", "--at", at20260224T1007, argFormat, "json"})
		if err != nil {
			t.Fatalf(errRunNext, err)
		}
	})
	var parsed nextResult
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf(errJSONParse, err)
	}
	if parsed.Count != 2 || len(parsed.Decisions) != 2 {
		t.Fatalf("unexpected next result: %+v", parsed)
	}

	err := runNext([]string{"backup", argCount, "1"})
	if err == nil || !strings.Contains(err.Error(), "--file is required") {
		t.Fatalf("expected required-file error, got %v", err)
	}

	err = runNext([]string{"backup", argFile, cfg, argCount, "0"})
	if err == nil || !strings.Contains(err.Error(), "--count must be > 0") {
		t.Fatalf("expected invalid count error, got %v", err)
	}

	err = runNext([]string{"backup", argFile, cfg, argCount, "1", "--at", "bad"})
	if err == nil || !strings.Contains(err.Error(), "invalid --at value") {
		t.Fatalf("expected invalid at error, got %v", err)
	}

	err = runNext([]string{"missing", argFile, cfg, argCount, "1", "--at", at20260224T1007})
	if !errors.Is(err, errJobNotFound) {
		t.Fatalf("expected errJobNotFound, got %v", err)
	}

	err = runNext([]string{"backup", argFile, cfg, argCount, "1", "--at", at20260224T1007, argFormat, "bad"})
	if err == nil || !strings.Contains(err.Error(), "invalid --format value") {
		t.Fatalf("expected invalid format error, got %v", err)
	}
}

func TestRunNextUnschedulableDecision(t *testing.T) {
	cfg := writeTempKrontab(t, `
*/30 * * * * @win(after,0s) @only(hours=9) name=backup command=/usr/bin/backup
`)

	stdout, _ := captureOutput(t, func() {
		err := runNext([]string{"backup", argFile, cfg, argCount, "1", "--at", at20260224T1007, argFormat, "json"})
		if err != nil {
			t.Fatalf(errRunNext, err)
		}
	})

	var parsed nextResult
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf(errJSONParse, err)
	}
	if len(parsed.Decisions) != 1 {
		t.Fatalf("expected one decision, got %d", len(parsed.Decisions))
	}
	if !parsed.Decisions[0].Unschedulable {
		t.Fatalf("expected unschedulable decision, got %+v", parsed.Decisions[0])
	}
	if parsed.Decisions[0].Reason == "" {
		t.Fatalf("expected unschedulable reason, got empty")
	}
}

func TestRunExplainConfigLoadError(t *testing.T) {
	cfg := writeTempKrontab(t, `
0 0 * * * @win(after,bad) name=backup command=/usr/bin/backup
`)

	err := runExplain([]string{"backup", argFile, cfg, "--at", at20260224T1000})
	if err == nil {
		t.Fatalf("expected config load error")
	}
	if !strings.Contains(err.Error(), "invalid @win duration") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunNextNoMatchingFuturePeriod(t *testing.T) {
	cfg := writeTempKrontab(t, `
0 0 31 2 * name=backup command=/usr/bin/backup
`)

	err := runNext([]string{"backup", argFile, cfg, argCount, "1", "--at", at20260224T1007, argFormat, "json"})
	if err == nil {
		t.Fatalf("expected no matching future period error")
	}
	if !strings.Contains(err.Error(), "no matching time found within 10 years") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDispatchExitMapping(t *testing.T) {
	cfg := writeTempKrontab(t, `*/30 * * * * @win(after,0s) @dist(uniform) name=backup command=/usr/bin/backup`)

	var code int
	_, stderr := captureOutput(t, func() {
		code = run([]string{"krontab", "explain", "backup", argFile, cfg, "--at", at20260224T1000})
	})
	if code != 0 {
		t.Fatalf(errExitCodeMismatchErr, code, 0, stderr)
	}

	_, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "next", "backup", argFile, cfg, argCount, "1", "--at", at20260224T1000})
	})
	if code != 0 {
		t.Fatalf(errExitCodeMismatchErr, code, 0, stderr)
	}

	_, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "explain", "missing", argFile, cfg, "--at", at20260224T1000})
	})
	if code != 1 {
		t.Fatalf(errExitCodeMismatchErr, code, 1, stderr)
	}

	_, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "explain", "backup"})
	})
	if code != 2 {
		t.Fatalf(errExitCodeMismatchErr, code, 2, stderr)
	}

	_, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "next", "missing", argFile, cfg, argCount, "1", "--at", at20260224T1000})
	})
	if code != 1 {
		t.Fatalf(errExitCodeMismatchErr, code, 1, stderr)
	}
}

func TestRunCanonicalErrorText(t *testing.T) {
	cfg := writeTempKrontab(t, `*/30 * * * * @win(after,0s) @dist(uniform) name=backup command=/usr/bin/backup`)

	var code int
	_, stderr := captureOutput(t, func() {
		code = run([]string{"krontab", "explain", "missing", argFile, cfg, "--at", at20260224T1000})
	})
	if code != 1 {
		t.Fatalf(errExitCodeMismatchErr, code, 1, stderr)
	}
	if stderr != "error: job not found: missing\n" {
		t.Fatalf(errStderrMismatch, stderr)
	}

	_, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "next", "backup"})
	})
	if code != 2 {
		t.Fatalf(errExitCodeMismatchErr, code, 2, stderr)
	}
	if stderr != "error: --file is required for MVP\n" {
		t.Fatalf(errStderrMismatch, stderr)
	}

	_, stderr = captureOutput(t, func() {
		code = run([]string{"krontab", "lint", argFile, cfg, "extra"})
	})
	if code != 2 {
		t.Fatalf(errExitCodeMismatchErr, code, 2, stderr)
	}
	if stderr != "error: lint does not accept positional arguments\n" {
		t.Fatalf(errStderrMismatch, stderr)
	}
}

func captureOutput(t *testing.T, fn func()) (stdout string, stderr string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	fn()

	_ = wOut.Close()
	_ = wErr.Close()

	outBytes, err := io.ReadAll(rOut)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	errBytes, err := io.ReadAll(rErr)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}

	return string(outBytes), string(errBytes)
}
