package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kron/core/pkg/core"
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

func TestExplainIntegrationModifierMatrix(t *testing.T) {
	tests := []struct {
		name            string
		config          string
		at              string
		wantCode        int
		wantErrContains string
		assert          func(t *testing.T, out string)
	}{
		{
			name: "tz_seed_dist_only_success",
			config: `
0 9 * * TUE @tz(America/New_York) @win(around,45m) @dist(skewLate,shape=3) @seed(daily,salt=team-a) @only(hours=9;dow=TUE) name=backup command=/usr/bin/backup
`,
			at:       "2026-02-24T14:00:00Z",
			wantCode: 0,
			assert: func(t *testing.T, out string) {
				var got core.Decision
				if err := json.Unmarshal([]byte(out), &got); err != nil {
					t.Fatalf("decode json output: %v", err)
				}
				if got.Mode != core.WindowModeCenter {
					t.Fatalf("mode mismatch: got %q", got.Mode)
				}
				if got.Distribution != core.DistributionSkewLate {
					t.Fatalf("distribution mismatch: got %q", got.Distribution)
				}
				if got.SeedStrategy != core.SeedStrategyDaily {
					t.Fatalf("seed strategy mismatch: got %q", got.SeedStrategy)
				}
				if got.PeriodKey != "2026-02-24" {
					t.Fatalf("period key mismatch: got %q", got.PeriodKey)
				}
				if got.Unschedulable {
					t.Fatalf("expected schedulable decision")
				}
			},
		},
		{
			name: "quoted_seed_salt_success",
			config: `
0 0 * * * @seed(stable,salt="team alpha") name=backup command=/usr/bin/backup
`,
			at:       "2026-02-24T00:00:00Z",
			wantCode: 0,
			assert: func(t *testing.T, out string) {
				var got core.Decision
				if err := json.Unmarshal([]byte(out), &got); err != nil {
					t.Fatalf("decode json output: %v", err)
				}
				if !strings.HasSuffix(got.SeedMaterial, "\nteam alpha") {
					t.Fatalf("seed material mismatch: got %q", got.SeedMaterial)
				}
			},
		},
		{
			name: "unsupported_normal_for_explain",
			config: `
0 9 * * * @dist(normal,mu=start,sigma=5m) name=backup command=/usr/bin/backup
`,
			at:              "2026-02-24T09:00:00Z",
			wantCode:        2,
			wantErrContains: `distribution "normal" is not supported in MVP explain`,
		},
		{
			name: "invalid_constraint_clause",
			config: `
	0 9 * * * @only(unknown=x) name=backup command=/usr/bin/backup
`,
			at:              "2026-02-24T09:00:00Z",
			wantCode:        2,
			wantErrContains: `unknown constraint clause`,
		},
		{
			name: "invalid_policy_value",
			config: `
	0 9 * * * @policy(concurrency=bad) name=backup command=/usr/bin/backup
`,
			at:              "2026-02-24T09:00:00Z",
			wantCode:        2,
			wantErrContains: `invalid concurrency "bad"`,
		},
		{
			name: "unknown_modifier_error",
			config: `
	0 9 * * * @oops(x=y) name=backup command=/usr/bin/backup
`,
			at:              "2026-02-24T09:00:00Z",
			wantCode:        2,
			wantErrContains: `unknown modifier @oops`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := writeTempKrontab(t, tt.config)
			stdout, stderr, code, err := runKrontab(
				"explain", "backup",
				"--file", cfg,
				"--at", tt.at,
				"--format", "json",
			)
			if err != nil {
				t.Fatalf("runKrontab error: %v", err)
			}
			if code != tt.wantCode {
				t.Fatalf("unexpected exit code: got %d want %d stderr=%q", code, tt.wantCode, stderr)
			}
			if tt.wantErrContains != "" && !strings.Contains(stderr, tt.wantErrContains) {
				t.Fatalf("expected stderr to contain %q, got %q", tt.wantErrContains, stderr)
			}
			if tt.assert != nil {
				tt.assert(t, stdout)
			}
		})
	}
}

func TestNextIntegrationModifierMatrix(t *testing.T) {
	tests := []struct {
		name            string
		config          string
		wantCode        int
		wantErrContains string
		assert          func(t *testing.T, out string)
	}{
		{
			name: "seed_tz_dist_only_success",
			config: `
*/30 * * * * @tz(UTC) @win(after,5m) @dist(skewEarly,shape=2) @seed(weekly,salt=ops) @only(dow=TUE;hours=10-23) name=backup command=/usr/bin/backup
`,
			wantCode: 0,
			assert: func(t *testing.T, out string) {
				var got nextResult
				if err := json.Unmarshal([]byte(out), &got); err != nil {
					t.Fatalf("decode json output: %v", err)
				}
				if len(got.Decisions) != 2 {
					t.Fatalf("expected 2 decisions, got %d", len(got.Decisions))
				}
				for i, d := range got.Decisions {
					if d.Distribution != core.DistributionSkewEarly {
						t.Fatalf("decision[%d] distribution mismatch: %q", i, d.Distribution)
					}
					if d.SeedStrategy != core.SeedStrategyWeekly {
						t.Fatalf("decision[%d] seed strategy mismatch: %q", i, d.SeedStrategy)
					}
					if d.Unschedulable {
						t.Fatalf("decision[%d] unexpectedly unschedulable", i)
					}
				}
			},
		},
		{
			name: "unschedulable_zero_window_constraint",
			config: `
*/30 * * * * @win(after,0s) @only(hours=9) name=backup command=/usr/bin/backup
`,
			wantCode: 0,
			assert: func(t *testing.T, out string) {
				var got nextResult
				if err := json.Unmarshal([]byte(out), &got); err != nil {
					t.Fatalf("decode json output: %v", err)
				}
				if len(got.Decisions) != 2 {
					t.Fatalf("expected 2 decisions, got %d", len(got.Decisions))
				}
				if !got.Decisions[0].Unschedulable || got.Decisions[0].Reason == "" {
					t.Fatalf("expected unschedulable decision with reason, got %+v", got.Decisions[0])
				}
			},
		},
		{
			name: "invalid_seed_parameter_error",
			config: `
	*/30 * * * * @seed(stable,foo=bar) name=backup command=/usr/bin/backup
`,
			wantCode:        2,
			wantErrContains: `unknown seed key "foo"`,
		},
		{
			name: "invalid_policy_parameter_error",
			config: `
	*/30 * * * * @policy(deadline=bad) name=backup command=/usr/bin/backup
`,
			wantCode:        2,
			wantErrContains: `invalid deadline "bad"`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := writeTempKrontab(t, tt.config)
			stdout, stderr, code, err := runKrontab(
				"next", "backup",
				"--file", cfg,
				"--count", "2",
				"--at", "2026-02-24T10:07:00Z",
				"--format", "json",
			)
			if err != nil {
				t.Fatalf("runKrontab error: %v", err)
			}
			if code != tt.wantCode {
				t.Fatalf("unexpected exit code: got %d want %d stderr=%q", code, tt.wantCode, stderr)
			}
			if tt.wantErrContains != "" && !strings.Contains(stderr, tt.wantErrContains) {
				t.Fatalf("expected stderr to contain %q, got %q", tt.wantErrContains, stderr)
			}
			if tt.assert != nil {
				tt.assert(t, stdout)
			}
		})
	}
}

func TestLintIntegrationModifierMatrix(t *testing.T) {
	validCfg := writeTempKrontab(t, `
0 10 * * * @tz(UTC) @win(after,2h) @dist(normal,mu=mid,sigma=10m) @seed(stable,salt="team alpha") @policy(concurrency=forbid,deadline=10m,suspend=false) @only(dow=MON-FRI;hours=8-18) name=backup command=/usr/bin/backup
`)
	stdout, stderr, code, err := runKrontab("lint", "--file", validCfg, "--format", "json")
	if err != nil {
		t.Fatalf("runKrontab lint valid error: %v", err)
	}
	if code != 0 {
		t.Fatalf("valid lint expected exit 0, got %d stderr=%q", code, stderr)
	}
	var valid lintResult
	if err := json.Unmarshal([]byte(stdout), &valid); err != nil {
		t.Fatalf("decode valid lint json: %v", err)
	}
	if !valid.Valid {
		t.Fatalf("expected valid lint result, got %+v", valid)
	}

	invalidCfg := writeTempKrontab(t, `
0 10 * * * @dist(normal,mu=bad,sigma=10m) name=backup command=/usr/bin/backup
`)
	stdout, stderr, code, err = runKrontab("lint", "--file", invalidCfg, "--format", "json")
	if err != nil {
		t.Fatalf("runKrontab lint invalid error: %v", err)
	}
	if code != 1 {
		t.Fatalf("invalid lint expected exit 1, got %d stderr=%q", code, stderr)
	}
	var invalid lintResult
	if err := json.Unmarshal([]byte(stdout), &invalid); err != nil {
		t.Fatalf("decode invalid lint json: %v", err)
	}
	if invalid.Valid {
		t.Fatalf("expected invalid lint result, got %+v", invalid)
	}
	found := false
	for _, e := range invalid.Errors {
		if strings.Contains(e, "invalid normal mu") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected invalid normal mu error, got %v", invalid.Errors)
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
