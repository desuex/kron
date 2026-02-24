package main

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"kron/core/pkg/core"
)

func TestLoadJobSettingsUsesModifiers(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * @win(around,30m) @dist(uniform) name=backup command=/usr/bin/backup
`)

	got, err := loadJobSettings(path, "backup", explainSettings{
		Window: time.Hour,
		Mode:   core.WindowModeAfter,
		Dist:   core.DistributionUniform,
	})
	if err != nil {
		t.Fatalf("loadJobSettings error: %v", err)
	}
	if got.Mode != core.WindowModeCenter {
		t.Fatalf("mode mismatch: got %s want %s", got.Mode, core.WindowModeCenter)
	}
	if got.Window != 30*time.Minute {
		t.Fatalf("window mismatch: got %s want %s", got.Window, 30*time.Minute)
	}
	if got.Dist != core.DistributionUniform {
		t.Fatalf("dist mismatch: got %s want %s", got.Dist, core.DistributionUniform)
	}
}

func TestLoadJobSettingsFallsBackWithoutModifiers(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * name=backup command=/usr/bin/backup
`)

	fallback := explainSettings{
		Window: 2 * time.Hour,
		Mode:   core.WindowModeBefore,
		Dist:   core.DistributionUniform,
	}
	got, err := loadJobSettings(path, "backup", fallback)
	if err != nil {
		t.Fatalf("loadJobSettings error: %v", err)
	}
	if got != fallback {
		t.Fatalf("settings mismatch: got %+v want %+v", got, fallback)
	}
}

func TestLoadJobSettingsNotFound(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * name=backup command=/usr/bin/backup
`)

	_, err := loadJobSettings(path, "missing", explainSettings{
		Window: time.Hour,
		Mode:   core.WindowModeAfter,
		Dist:   core.DistributionUniform,
	})
	if !errors.Is(err, errJobNotFound) {
		t.Fatalf("expected errJobNotFound, got: %v", err)
	}
}

func TestLoadJobSettingsRejectsUnsupportedDistribution(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * @dist(normal) name=backup command=/usr/bin/backup
`)

	_, err := loadJobSettings(path, "backup", explainSettings{
		Window: time.Hour,
		Mode:   core.WindowModeAfter,
		Dist:   core.DistributionUniform,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadJobSettingsDuplicateJob(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * name=backup command=/usr/bin/backup
5 0 * * * name=backup command=/usr/bin/backup2
`)

	_, err := loadJobSettings(path, "backup", explainSettings{
		Window: time.Hour,
		Mode:   core.WindowModeAfter,
		Dist:   core.DistributionUniform,
	})
	if err == nil {
		t.Fatalf("expected duplicate job error")
	}
}

func TestLoadJobDefinitionIncludesScheduleAndTimezone(t *testing.T) {
	path := writeTempKrontab(t, `
0 9 * * * @tz(America/New_York) @win(after,0s) name=backup command=/usr/bin/backup
`)

	def, err := loadJobDefinition(path, "backup", explainSettings{
		Window: 0,
		Mode:   core.WindowModeAfter,
		Dist:   core.DistributionUniform,
	})
	if err != nil {
		t.Fatalf("loadJobDefinition error: %v", err)
	}

	next, err := def.Schedule.NextAfter(time.Date(2026, 2, 24, 13, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NextAfter error: %v", err)
	}
	want := time.Date(2026, 2, 24, 14, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next mismatch: got %s want %s", next, want)
	}
}

func TestConfigHelpers(t *testing.T) {
	if _, err := findFieldStart([]string{"0", "0", "*", "*"}); err == nil {
		t.Fatalf("expected missing fields error")
	}
	if _, err := findFieldStart([]string{"0", "0", "*", "name=x"}); err == nil {
		t.Fatalf("expected invalid cron expression error")
	}
	i, err := findFieldStart([]string{"0", "0", "*", "*", "*", "name=x"})
	if err != nil || i != 5 {
		t.Fatalf("unexpected findFieldStart result: i=%d err=%v", i, err)
	}

	if got := extractName([]string{"command=/bin/echo"}); got != "" {
		t.Fatalf("expected empty name, got %q", got)
	}
	if got := extractName([]string{"name=backup", "command=/bin/echo"}); got != "backup" {
		t.Fatalf("unexpected name: %q", got)
	}

	if mode, err := mapWindowMode("after"); err != nil || mode != core.WindowModeAfter {
		t.Fatalf("after mapping failed: %v %v", mode, err)
	}
	if mode, err := mapWindowMode("around"); err != nil || mode != core.WindowModeCenter {
		t.Fatalf("around mapping failed: %v %v", mode, err)
	}
	if mode, err := mapWindowMode("center"); err != nil || mode != core.WindowModeCenter {
		t.Fatalf("center mapping failed: %v %v", mode, err)
	}
	if mode, err := mapWindowMode("before"); err != nil || mode != core.WindowModeBefore {
		t.Fatalf("before mapping failed: %v %v", mode, err)
	}
	if _, err := mapWindowMode("bad"); err == nil {
		t.Fatalf("expected invalid mode error")
	}
}

func TestParseExplainModifiersErrors(t *testing.T) {
	fallback := explainSettings{
		Window: time.Hour,
		Mode:   core.WindowModeAfter,
		Dist:   core.DistributionUniform,
	}

	if _, err := parseExplainModifiers([]string{"@win(after)"}, fallback); err == nil {
		t.Fatalf("expected invalid @win args error")
	}
	if _, err := parseExplainModifiers([]string{"@win(bad,1h)"}, fallback); err == nil {
		t.Fatalf("expected invalid @win mode error")
	}
	if _, err := parseExplainModifiers([]string{"@win(after,bad)"}, fallback); err == nil {
		t.Fatalf("expected invalid @win duration error")
	}
	if _, err := parseExplainModifiers([]string{"@dist()"}, fallback); err == nil {
		t.Fatalf("expected invalid @dist args error")
	}
	if _, err := parseExplainModifiers([]string{"@dist(normal)"}, fallback); err == nil {
		t.Fatalf("expected unsupported distribution error")
	}
}

func TestLoadJobDefinitionInvalidLine(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * @win(after,bad) name=backup command=/bin/echo
`)
	_, err := loadJobDefinition(path, "backup", explainSettings{
		Window: time.Hour,
		Mode:   core.WindowModeAfter,
		Dist:   core.DistributionUniform,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid @win duration") {
		t.Fatalf("expected invalid win duration error, got %v", err)
	}
}

func writeTempKrontab(t *testing.T, content string) string {
	t.Helper()

	f, err := os.CreateTemp("", "krontab-*.kron")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return f.Name()
}
