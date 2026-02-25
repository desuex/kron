package daemon

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kron/core/pkg/core"
)

func TestMapWindowModeAndSplitModifierErrors(t *testing.T) {
	valid := []string{"after", "around", "center", "before"}
	for _, mode := range valid {
		if _, err := mapWindowMode(mode); err != nil {
			t.Fatalf("mapWindowMode(%q) unexpected error: %v", mode, err)
		}
	}
	if _, err := mapWindowMode("bad"); err == nil {
		t.Fatalf("expected invalid mode error")
	}

	tests := []string{
		"tz(UTC)",
		"@tzUTC)",
		"@tz(",
		"@only()",
	}
	for _, tok := range tests {
		if _, _, err := splitModifier(tok); err == nil {
			t.Fatalf("expected splitModifier error for %q", tok)
		}
	}
}

func TestParseSeedDistPolicyErrors(t *testing.T) {
	if _, _, err := parseSeedModifier(""); err == nil {
		t.Fatalf("expected empty seed error")
	}
	if _, _, err := parseSeedModifier("stable,foo=bar"); err == nil {
		t.Fatalf("expected unknown seed key error")
	}

	distErrors := []string{
		"",
		"normal",
		"uniform,shape=2",
		"skewLate,foo=1",
		"skewLate,shape=0",
	}
	for _, body := range distErrors {
		if _, _, err := parseDistModifier(body); err == nil {
			t.Fatalf("expected parseDistModifier error for %q", body)
		}
	}

	policyErrors := []string{
		"concurrency=bad",
		"deadline=bad",
		"suspend=bad",
		"foo=bar",
		"bad",
	}
	for _, body := range policyErrors {
		if _, err := parsePolicyModifier(body); err == nil {
			t.Fatalf("expected parsePolicyModifier error for %q", body)
		}
	}
}

func TestFindFieldStartAndSplitTokensErrors(t *testing.T) {
	if _, err := findFieldStart([]string{"*", "*"}); err == nil {
		t.Fatalf("expected missing field error")
	}
	if _, err := findFieldStart([]string{"*", "*", "*", "*", "name=x"}); err == nil {
		t.Fatalf("expected invalid cron expression error")
	}

	if _, err := splitTokens(`0 0 * * * name=x command="/bin/echo`); err == nil {
		t.Fatalf("expected unterminated quote error")
	}
	if _, err := splitTokens("   "); err == nil {
		t.Fatalf("expected empty entry error")
	}
	if _, err := splitTokens(`0 0 * * * name=x command="abc\`); err == nil {
		t.Fatalf("expected invalid escape at end error")
	}
}

func TestParseJobFieldsAcceptsCompatibilityFields(t *testing.T) {
	job := JobConfig{}
	err := parseJobFields(&job, []string{
		"name=backup",
		"command=true",
		"user=root",
		"group=wheel",
		"umask=0022",
		"stdout=inherit",
		"stderr=discard",
		"description=backup",
		"cwd=/tmp",
		"shell=true",
		"timeout=5s",
		"env=MODE=prod",
	})
	if err != nil {
		t.Fatalf("parseJobFields error: %v", err)
	}
	if !job.Command.Shell || job.Command.Timeout != 5*time.Second || job.Command.Cwd != "/tmp" {
		t.Fatalf("command field parse mismatch: %+v", job.Command)
	}
}

func TestLoadJobsDirAndFileErrors(t *testing.T) {
	emptyDir := t.TempDir()
	if _, err := LoadJobs(emptyDir); err == nil {
		t.Fatalf("expected no jobs in dir error")
	}

	fileNoJobs := writeTempConfig(t, "# comment only\n\n")
	if _, err := LoadJobs(fileNoJobs); err == nil {
		t.Fatalf("expected no jobs in file error")
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.kron"), "0 0 * * * name=dup command=true\n")
	writeFile(t, filepath.Join(dir, "b.kron"), "0 1 * * * name=dup command=true\n")
	jobs, err := LoadJobs(dir)
	if err != nil {
		t.Fatalf("duplicate names across files should be allowed (identity includes path): %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("unexpected jobs count: %d", len(jobs))
	}
}

func TestParseJobModifiersUnknownAndConstraints(t *testing.T) {
	job := JobConfig{
		Mode:     core.WindowModeAfter,
		Dist:     core.DistributionUniform,
		Timezone: "UTC",
		Seed:     core.SeedStrategyStable,
		Policy: PolicySpec{
			Concurrency: DefaultConcurrency,
		},
	}
	if err := parseJobModifiers(&job, []string{"@unknown(x=1)"}); err == nil {
		t.Fatalf("expected unknown modifier error")
	}
	if err := parseJobModifiers(&job, []string{"@only(hours=8-9)", "@avoid(date=2026-03-01)"}); err != nil {
		t.Fatalf("expected constraint modifiers success: %v", err)
	}
	if len(job.Constraints.OnlyHours) == 0 || len(job.Constraints.AvoidDates) == 0 {
		t.Fatalf("constraint parse mismatch: %+v", job.Constraints)
	}

	err := parseJobModifiers(&job, []string{"@tz(No/Such_TZ)"})
	if err == nil || !strings.Contains(err.Error(), "invalid timezone") {
		t.Fatalf("expected timezone error, got %v", err)
	}
}
