package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kron/core/pkg/core"
)

func TestLoadJobsFromFile(t *testing.T) {
	path := writeTempConfig(t, `
# backup
0 0 * * * @tz(UTC) @win(after,30m) @dist(skewEarly,shape=2) @seed(daily,salt=team-a) @only(hours=0;dow=THU) @policy(concurrency=forbid,deadline=10m) name=backup command="/usr/bin/backup --full" shell=false timeout=1m env=MODE=prod
`)

	jobs, err := LoadJobs(path)
	if err != nil {
		t.Fatalf("LoadJobs error: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs length mismatch: got %d want 1", len(jobs))
	}

	job := jobs[0]
	if job.Name != "backup" {
		t.Fatalf("name mismatch: %q", job.Name)
	}
	if !strings.HasSuffix(job.Identity, ":"+job.Name) {
		t.Fatalf("identity mismatch: %q", job.Identity)
	}
	if job.Mode != core.WindowModeAfter || job.Window != 30*time.Minute {
		t.Fatalf("window mismatch: mode=%s window=%s", job.Mode, job.Window)
	}
	if job.Dist != core.DistributionSkewEarly || job.SkewShape != 2 {
		t.Fatalf("distribution mismatch: dist=%s shape=%v", job.Dist, job.SkewShape)
	}
	if job.Seed != core.SeedStrategyDaily || job.Salt != "team-a" {
		t.Fatalf("seed mismatch: seed=%s salt=%q", job.Seed, job.Salt)
	}
	if job.Policy.Concurrency != "forbid" || job.Policy.Deadline != 10*time.Minute {
		t.Fatalf("policy mismatch: %+v", job.Policy)
	}
	if len(job.Constraints.OnlyHours) != 1 || job.Constraints.OnlyHours[0] != 0 {
		t.Fatalf("only-hours mismatch: %+v", job.Constraints.OnlyHours)
	}
	if len(job.Constraints.OnlyDOW) != 1 || job.Constraints.OnlyDOW[0] != 4 {
		t.Fatalf("only-dow mismatch: %+v", job.Constraints.OnlyDOW)
	}
	if job.Command.Raw != "/usr/bin/backup --full" {
		t.Fatalf("command raw mismatch: %q", job.Command.Raw)
	}
	if job.Command.Shell {
		t.Fatalf("shell mismatch: got true want false")
	}
	if job.Command.Timeout != time.Minute {
		t.Fatalf("timeout mismatch: %s", job.Command.Timeout)
	}
	if len(job.Command.Env) != 1 || job.Command.Env[0] != "MODE=prod" {
		t.Fatalf("env mismatch: %+v", job.Command.Env)
	}
}

func TestLoadJobsDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.kron"), "0 0 * * * name=a command=/bin/true\n")
	writeFile(t, filepath.Join(dir, "b.kron"), "0 0 * * * name=b command=/bin/true\n")

	jobs, err := LoadJobs(dir)
	if err != nil {
		t.Fatalf("LoadJobs dir error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("jobs length mismatch: got %d want 2", len(jobs))
	}
}

func TestLoadJobsRejectsUnsupportedRuntimeDistribution(t *testing.T) {
	path := writeTempConfig(t, `0 0 * * * @dist(normal) name=backup command=/bin/true`)
	_, err := LoadJobs(path)
	if err == nil || !strings.Contains(err.Error(), "not supported in current krond runtime") {
		t.Fatalf("expected unsupported runtime dist error, got %v", err)
	}
}

func TestLoadJobsRejectsUnknownField(t *testing.T) {
	path := writeTempConfig(t, `0 0 * * * name=backup command=/bin/true badfield=x`)
	_, err := LoadJobs(path)
	if err == nil || !strings.Contains(err.Error(), `unknown field "badfield"`) {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "krond-*.kron")
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
