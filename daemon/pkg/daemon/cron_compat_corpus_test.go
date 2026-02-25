package daemon

import (
	"path/filepath"
	goRuntime "runtime"
	"strings"
	"testing"
	"time"
)

func TestCronCompatibilityCorpusSystemCrontab(t *testing.T) {
	path := corpusPath(t, "system.crontab")
	jobs, err := LoadSystemCron(path)
	if err != nil {
		t.Fatalf("LoadSystemCron corpus file error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("job count mismatch: got %d want 3", len(jobs))
	}

	complex := findJobByCommand(t, jobs, "weekday-morning")
	if !complex.Command.Shell {
		t.Fatalf("expected shell execution mode for cron corpus command")
	}
	if !containsEnv(complex.Command.Env, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin") {
		t.Fatalf("expected PATH env inheritance, got %+v", complex.Command.Env)
	}
	if !containsEnv(complex.Command.Env, "CRON_TZ=UTC") {
		t.Fatalf("expected CRON_TZ env inheritance, got %+v", complex.Command.Env)
	}

	next, err := complex.Schedule.NextAfter(time.Date(2026, 1, 1, 7, 59, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NextAfter for complex syntax error: %v", err)
	}
	if next.Minute()%10 != 0 {
		t.Fatalf("expected stepped minute, got %s", next)
	}
	if next.Hour() < 8 || next.Hour() > 10 {
		t.Fatalf("expected hour in 8-10 range, got %s", next)
	}
	if next.Month() != time.January && next.Month() != time.March {
		t.Fatalf("expected month in JAN,MAR, got %s", next)
	}
	if next.Weekday() < time.Monday || next.Weekday() > time.Friday {
		t.Fatalf("expected weekday MON-FRI, got %s", next)
	}

	domDow := findJobByCommand(t, jobs, "dom-dow-or")
	assertDayOfMonthOrDayOfWeekSemantics(t, domDow.Schedule)

	hourly := findJobByCommand(t, jobs, "hourly")
	hourlyNext, err := hourly.Schedule.NextAfter(time.Date(2026, 3, 1, 10, 15, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NextAfter for @hourly error: %v", err)
	}
	if hourlyNext.Minute() != 0 {
		t.Fatalf("expected hourly minute 0, got %s", hourlyNext)
	}
}

func TestCronCompatibilityCorpusCronDDirectory(t *testing.T) {
	dir := corpusPath(t, "cron.d")
	jobs, err := LoadSystemCron(dir)
	if err != nil {
		t.Fatalf("LoadSystemCron corpus dir error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("job count mismatch: got %d want 2", len(jobs))
	}

	appJob := findJobByCommand(t, jobs, "/opt/app/job")
	if appJob.Command.User != "app" {
		t.Fatalf("expected parsed cron user app, got %q", appJob.Command.User)
	}
	if appJob.Timezone != "America/New_York" {
		t.Fatalf("expected TZ from file env, got %q", appJob.Timezone)
	}
	if !containsEnv(appJob.Command.Env, "MAILTO=ops@example.com") || !containsEnv(appJob.Command.Env, "TZ=America/New_York") {
		t.Fatalf("missing app env values: %+v", appJob.Command.Env)
	}
	if !strings.HasSuffix(appJob.Identity, ":app") {
		t.Fatalf("expected user marker in identity, got %q", appJob.Identity)
	}

	dbJob := findJobByCommand(t, jobs, "/opt/db/maint")
	if dbJob.Command.User != "postgres" {
		t.Fatalf("expected parsed cron user postgres, got %q", dbJob.Command.User)
	}
	if dbJob.Timezone != "UTC" {
		t.Fatalf("expected default UTC timezone for db file, got %q", dbJob.Timezone)
	}
	if containsEnv(dbJob.Command.Env, "MAILTO=ops@example.com") || containsEnv(dbJob.Command.Env, "TZ=America/New_York") {
		t.Fatalf("unexpected env bleed across cron.d files: %+v", dbJob.Command.Env)
	}
	if !strings.HasSuffix(dbJob.Identity, ":postgres") {
		t.Fatalf("expected postgres identity marker, got %q", dbJob.Identity)
	}
}

func assertDayOfMonthOrDayOfWeekSemantics(t *testing.T, schedule CronSpec) {
	t.Helper()

	domOnly := findScheduledTime(t, func(ts time.Time) bool {
		return ts.Day() == 1 && ts.Weekday() != time.Monday
	})
	if !schedule.matches(domOnly) {
		t.Fatalf("expected match for DOM-only day, got %s", domOnly)
	}

	dowOnly := findScheduledTime(t, func(ts time.Time) bool {
		return ts.Weekday() == time.Monday && ts.Day() != 1
	})
	if !schedule.matches(dowOnly) {
		t.Fatalf("expected match for DOW-only day, got %s", dowOnly)
	}

	neither := findScheduledTime(t, func(ts time.Time) bool {
		return ts.Day() != 1 && ts.Weekday() != time.Monday
	})
	if schedule.matches(neither) {
		t.Fatalf("expected no match when both DOM and DOW miss, got %s", neither)
	}
}

func findScheduledTime(t *testing.T, cond func(time.Time) bool) time.Time {
	t.Helper()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 400; i++ {
		candidate := start.AddDate(0, 0, i)
		if cond(candidate) {
			return candidate
		}
	}
	t.Fatalf("failed to find compatible time in scan window")
	return time.Time{}
}

func findJobByCommand(t *testing.T, jobs []JobConfig, contains string) JobConfig {
	t.Helper()
	for _, job := range jobs {
		if strings.Contains(job.Command.Raw, contains) {
			return job
		}
	}
	t.Fatalf("job containing %q not found", contains)
	return JobConfig{}
}

func corpusPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, thisFile, _, ok := goRuntime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	base := filepath.Join(filepath.Dir(thisFile), "testdata", "cron_corpus")
	all := append([]string{base}, elems...)
	return filepath.Join(all...)
}
