package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadSystemCronFile(t *testing.T) {
	path := writeTempCronFile(t, strings.Join([]string{
		`SHELL="/bin/bash"`,
		`CRON_TZ=America/New_York`,
		`*/15 9 * * MON-FRI root /usr/bin/echo hello world`,
		`@daily root /usr/bin/backup --full`,
	}, "\n")+"\n")

	jobs, err := LoadSystemCron(path)
	if err != nil {
		t.Fatalf("LoadSystemCron error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("job count mismatch: got %d want 2", len(jobs))
	}

	first := jobs[0]
	if first.Timezone != "America/New_York" {
		t.Fatalf("timezone mismatch: got %q", first.Timezone)
	}
	if !first.Command.Shell {
		t.Fatalf("expected shell mode for system cron entry")
	}
	if first.Command.Raw != "/usr/bin/echo hello world" {
		t.Fatalf("command mismatch: %q", first.Command.Raw)
	}
	if first.Command.User != "root" {
		t.Fatalf("user mismatch: got %q want root", first.Command.User)
	}
	if !containsEnv(first.Command.Env, "SHELL=/bin/bash") || !containsEnv(first.Command.Env, "CRON_TZ=America/New_York") {
		t.Fatalf("missing inherited env: %+v", first.Command.Env)
	}

	next, err := first.Schedule.NextAfter(time.Date(2026, 2, 24, 13, 0, 0, 0, time.UTC)) // 08:00 EST
	if err != nil {
		t.Fatalf("NextAfter error: %v", err)
	}
	want := time.Date(2026, 2, 24, 14, 0, 0, 0, time.UTC) // 09:00 EST
	if !next.Equal(want) {
		t.Fatalf("next mismatch: got %s want %s", next, want)
	}
}

func TestLoadSystemCronDir(t *testing.T) {
	dir := t.TempDir()
	writeCronFileAt(t, filepath.Join(dir, "a"), "0 * * * * root /bin/true\n")
	writeCronFileAt(t, filepath.Join(dir, "b"), "30 * * * * root /bin/echo hi\n")
	writeCronFileAt(t, filepath.Join(dir, ".ignore"), "0 0 * * * root /bin/false\n")

	jobs, err := LoadSystemCron(dir)
	if err != nil {
		t.Fatalf("LoadSystemCron dir error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("job count mismatch: got %d want 2", len(jobs))
	}
	if !strings.Contains(jobs[0].Identity, "/a:1:root") || !strings.Contains(jobs[1].Identity, "/b:1:root") {
		t.Fatalf("unexpected identities: %q %q", jobs[0].Identity, jobs[1].Identity)
	}
}

func TestLoadSystemCronRejectsInvalidEntries(t *testing.T) {
	invalid := writeTempCronFile(t, "not a cron entry\n")
	if _, err := LoadSystemCron(invalid); err == nil {
		t.Fatalf("expected parse error for invalid entry")
	}

	reboot := writeTempCronFile(t, "@reboot root /bin/true\n")
	if _, err := LoadSystemCron(reboot); err == nil {
		t.Fatalf("expected unsupported macro error")
	}

	badTZ := writeTempCronFile(t, "CRON_TZ=No/Such_TZ\n0 0 * * * root /bin/true\n")
	if _, err := LoadSystemCron(badTZ); err == nil {
		t.Fatalf("expected invalid timezone error")
	}
}

func TestLoadSystemCronErrorPaths(t *testing.T) {
	if _, err := LoadSystemCron(filepath.Join(t.TempDir(), "missing.cron")); err == nil {
		t.Fatalf("expected stat path error")
	}

	emptyDir := t.TempDir()
	if _, err := LoadSystemCron(emptyDir); err == nil {
		t.Fatalf("expected empty cron dir error")
	}

	commentsOnly := writeTempCronFile(t, "# nothing\n\n")
	if _, err := LoadSystemCron(commentsOnly); err == nil {
		t.Fatalf("expected no jobs found error")
	}
}

func TestParseSystemCronEntryMacroAndHelpers(t *testing.T) {
	fields, user, cmd, err := parseSystemCronEntry("@hourly root /usr/bin/date")
	if err != nil {
		t.Fatalf("parseSystemCronEntry macro error: %v", err)
	}
	if fields != [5]string{"0", "*", "*", "*", "*"} || user != "root" || cmd != "/usr/bin/date" {
		t.Fatalf("unexpected macro parse result: fields=%v user=%q cmd=%q", fields, user, cmd)
	}

	key, value, err := parseEnvAssignment(`MAILTO="ops@example.com"`)
	if err != nil {
		t.Fatalf("parseEnvAssignment error: %v", err)
	}
	if key != "MAILTO" || value != "ops@example.com" {
		t.Fatalf("unexpected env assignment parse result: %q=%q", key, value)
	}

	if got := sanitizeJobPart("Root.User"); got != "root-user" {
		t.Fatalf("sanitizeJobPart mismatch: got %q", got)
	}
}

func TestParseSystemCronEntryStandardAndEnvErrors(t *testing.T) {
	fields, user, cmd, err := parseSystemCronEntry("5 4 * * * daemon /usr/bin/job --run")
	if err != nil {
		t.Fatalf("parseSystemCronEntry standard error: %v", err)
	}
	if fields != [5]string{"5", "4", "*", "*", "*"} || user != "daemon" || cmd != "/usr/bin/job --run" {
		t.Fatalf("unexpected standard parse result: fields=%v user=%q cmd=%q", fields, user, cmd)
	}

	if _, _, _, err := parseSystemCronEntry("@foo root /bin/true"); err == nil {
		t.Fatalf("expected unsupported macro error")
	}
	if _, _, _, err := parseSystemCronEntry("bad entry"); err == nil {
		t.Fatalf("expected invalid entry error")
	}

	if _, _, err := parseEnvAssignment("NO_EQUALS"); err == nil {
		t.Fatalf("expected invalid env assignment error")
	}
	if _, _, err := parseEnvAssignment(" =value"); err == nil {
		t.Fatalf("expected empty env key error")
	}
}

func TestOrderedEnvTimezoneFallback(t *testing.T) {
	env := newOrderedEnv()
	if got := env.Timezone(); got != "UTC" {
		t.Fatalf("expected UTC fallback, got %q", got)
	}

	env.Set("TZ", "America/Los_Angeles")
	if got := env.Timezone(); got != "America/Los_Angeles" {
		t.Fatalf("expected TZ fallback, got %q", got)
	}

	env.Set("CRON_TZ", "Europe/Berlin")
	if got := env.Timezone(); got != "Europe/Berlin" {
		t.Fatalf("expected CRON_TZ precedence, got %q", got)
	}

	if got := sanitizeJobPart("   "); got != "job" {
		t.Fatalf("expected empty sanitize fallback, got %q", got)
	}
	if got := sanitizeJobPart("###"); got != "job" {
		t.Fatalf("expected punctuation sanitize fallback, got %q", got)
	}
}

func writeTempCronFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "system.cron")
	writeCronFileAt(t, path, content)
	return path
}

func writeCronFileAt(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write cron file %s: %v", path, err)
	}
}

func containsEnv(env []string, item string) bool {
	for _, e := range env {
		if e == item {
			return true
		}
	}
	return false
}
