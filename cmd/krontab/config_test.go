package main

import (
	"errors"
	"os"
	"reflect"
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
		Window:       time.Hour,
		Mode:         core.WindowModeAfter,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
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

func TestLoadJobSettingsAcceptsSkewDistribution(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * @win(after,30m) @dist(skewEarly,shape=2.5) name=backup command=/usr/bin/backup
`)

	got, err := loadJobSettings(path, "backup", explainSettings{
		Window:       time.Hour,
		Mode:         core.WindowModeAfter,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
	})
	if err != nil {
		t.Fatalf("loadJobSettings error: %v", err)
	}
	if got.Dist != core.DistributionSkewEarly {
		t.Fatalf("dist mismatch: got %s want %s", got.Dist, core.DistributionSkewEarly)
	}
	if got.SkewShape != 2.5 {
		t.Fatalf("skew shape mismatch: got %v want %v", got.SkewShape, 2.5)
	}
}

func TestLoadJobSettingsParsesTimezoneAndSeed(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * @tz(America/New_York) @seed(daily,salt=team-a) name=backup command=/usr/bin/backup
`)

	got, err := loadJobSettings(path, "backup", explainSettings{
		Window:       time.Hour,
		Mode:         core.WindowModeAfter,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
	})
	if err != nil {
		t.Fatalf("loadJobSettings error: %v", err)
	}
	if got.Timezone != "America/New_York" {
		t.Fatalf("timezone mismatch: got %q want %q", got.Timezone, "America/New_York")
	}
	if got.SeedStrategy != core.SeedStrategyDaily {
		t.Fatalf("seed strategy mismatch: got %q want %q", got.SeedStrategy, core.SeedStrategyDaily)
	}
	if got.Salt != "team-a" {
		t.Fatalf("salt mismatch: got %q want %q", got.Salt, "team-a")
	}
}

func TestLoadJobSettingsParsesPolicy(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * @policy(concurrency=replace,deadline=10m,suspend=true) name=backup command=/usr/bin/backup
`)

	got, err := loadJobSettings(path, "backup", explainSettings{
		Window:       time.Hour,
		Mode:         core.WindowModeAfter,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
	})
	if err != nil {
		t.Fatalf("loadJobSettings error: %v", err)
	}
	if got.Policy.Concurrency != "replace" {
		t.Fatalf("policy concurrency mismatch: got %q want %q", got.Policy.Concurrency, "replace")
	}
	if got.Policy.Deadline != 10*time.Minute {
		t.Fatalf("policy deadline mismatch: got %s want %s", got.Policy.Deadline, 10*time.Minute)
	}
	if !got.Policy.Suspend {
		t.Fatalf("policy suspend mismatch: got %v want true", got.Policy.Suspend)
	}
}

func TestLoadJobSettingsParsesQuotedSeedSalt(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * @seed(stable,salt="team alpha") name=backup command=/usr/bin/backup
`)

	got, err := loadJobSettings(path, "backup", explainSettings{
		Window:       time.Hour,
		Mode:         core.WindowModeAfter,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
	})
	if err != nil {
		t.Fatalf("loadJobSettings error: %v", err)
	}
	if got.Salt != "team alpha" {
		t.Fatalf("quoted salt mismatch: got %q want %q", got.Salt, "team alpha")
	}
}

func TestLoadJobSettingsParsesOnlyAvoidConstraints(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * @only(hours=8-10;dow=MON-FRI;dom=1-5;months=JAN-MAR;date=2026-03-02) @avoid(between=09:30-09:45;dates=2026-03-10..2026-03-12) name=backup command=/usr/bin/backup
`)

	got, err := loadJobSettings(path, "backup", explainSettings{
		Window:       time.Hour,
		Mode:         core.WindowModeAfter,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
		Constraints:  core.ConstraintSpec{},
	})
	if err != nil {
		t.Fatalf("loadJobSettings error: %v", err)
	}
	if len(got.Constraints.OnlyHours) == 0 ||
		len(got.Constraints.OnlyDOW) == 0 ||
		len(got.Constraints.OnlyDOM) == 0 ||
		len(got.Constraints.OnlyMonths) == 0 ||
		len(got.Constraints.OnlyDates) == 0 ||
		len(got.Constraints.AvoidBetween) == 0 ||
		len(got.Constraints.AvoidDates) == 0 {
		t.Fatalf("expected parsed constraints, got %+v", got.Constraints)
	}
}

func TestLoadJobSettingsFallsBackWithoutModifiers(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * name=backup command=/usr/bin/backup
`)

	fallback := explainSettings{
		Window:       2 * time.Hour,
		Mode:         core.WindowModeBefore,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
	}
	got, err := loadJobSettings(path, "backup", fallback)
	if err != nil {
		t.Fatalf("loadJobSettings error: %v", err)
	}
	if !reflect.DeepEqual(got, fallback) {
		t.Fatalf("settings mismatch: got %+v want %+v", got, fallback)
	}
}

func TestLoadJobSettingsNotFound(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * name=backup command=/usr/bin/backup
`)

	_, err := loadJobSettings(path, "missing", explainSettings{
		Window:       time.Hour,
		Mode:         core.WindowModeAfter,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
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
		Window:       time.Hour,
		Mode:         core.WindowModeAfter,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
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
		Window:       time.Hour,
		Mode:         core.WindowModeAfter,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
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
		Window:       0,
		Mode:         core.WindowModeAfter,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
	})
	if err != nil {
		t.Fatalf("loadJobDefinition error: %v", err)
	}
	if def.Settings.Timezone != "America/New_York" {
		t.Fatalf("timezone mismatch: got %q", def.Settings.Timezone)
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
		Window:       time.Hour,
		Mode:         core.WindowModeAfter,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
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
	if _, err := parseExplainModifiers([]string{"@dist(skewLate,shape=bad)"}, fallback); err == nil {
		t.Fatalf("expected invalid shape error")
	}
	if _, err := parseExplainModifiers([]string{"@dist(skewLate,foo=1)"}, fallback); err == nil {
		t.Fatalf("expected unknown skew parameter error")
	}
	if _, err := parseExplainModifiers([]string{"@dist(uniform,shape=2)"}, fallback); err == nil {
		t.Fatalf("expected uniform parameter error")
	}
	if _, err := parseExplainModifiers([]string{"@seed(stable,foo=bar)"}, fallback); err == nil {
		t.Fatalf("expected unknown seed key error")
	}
	if _, err := parseExplainModifiers([]string{"@tz(Not/AZone)"}, fallback); err == nil {
		t.Fatalf("expected invalid timezone error")
	}
	if _, err := parseExplainModifiers([]string{"@policy(concurrency=bad)"}, fallback); err == nil {
		t.Fatalf("expected invalid policy concurrency error")
	}
	if _, err := parseExplainModifiers([]string{"@unknown(x=y)"}, fallback); err == nil {
		t.Fatalf("expected unknown modifier error")
	}
}

func TestParseDistModifier(t *testing.T) {
	dist, shape, err := parseDistModifier("uniform")
	if err != nil {
		t.Fatalf("parseDistModifier uniform error: %v", err)
	}
	if dist != core.DistributionUniform || shape != 0 {
		t.Fatalf("uniform parse mismatch: dist=%q shape=%v", dist, shape)
	}

	dist, shape, err = parseDistModifier("skewLate,shape=3.5")
	if err != nil {
		t.Fatalf("parseDistModifier skew error: %v", err)
	}
	if dist != core.DistributionSkewLate || shape != 3.5 {
		t.Fatalf("skew parse mismatch: dist=%q shape=%v", dist, shape)
	}

	invalid := []string{
		"skewLate,shape=",
		"skewLate,shape=0",
		"skewLate,shape=-1",
		"skewLate,badparam",
	}
	for _, tt := range invalid {
		if _, _, err := parseDistModifier(tt); err == nil {
			t.Fatalf("expected parseDistModifier error for %q", tt)
		}
	}
}

func TestParseSeedModifier(t *testing.T) {
	strategy, salt, err := parseSeedModifier("weekly,salt=team-x")
	if err != nil {
		t.Fatalf("parseSeedModifier error: %v", err)
	}
	if strategy != core.SeedStrategyWeekly {
		t.Fatalf("strategy mismatch: got %q want %q", strategy, core.SeedStrategyWeekly)
	}
	if salt != "team-x" {
		t.Fatalf("salt mismatch: got %q want %q", salt, "team-x")
	}

	tests := []string{
		"",
		"bad",
		"stable,badparam",
		"daily,foo=bar",
	}
	for _, tt := range tests {
		if _, _, err := parseSeedModifier(tt); err == nil {
			t.Fatalf("expected parseSeedModifier error for %q", tt)
		}
	}
}

func TestParsePolicyModifier(t *testing.T) {
	policy, err := parsePolicyModifier("concurrency=allow,deadline=5m,suspend=true")
	if err != nil {
		t.Fatalf("parsePolicyModifier error: %v", err)
	}
	if policy.Concurrency != "allow" {
		t.Fatalf("policy concurrency mismatch: got %q want %q", policy.Concurrency, "allow")
	}
	if policy.Deadline != 5*time.Minute {
		t.Fatalf("policy deadline mismatch: got %s want %s", policy.Deadline, 5*time.Minute)
	}
	if !policy.Suspend {
		t.Fatalf("policy suspend mismatch: got %v want true", policy.Suspend)
	}

	tests := []string{
		"",
		"badparam",
		"concurrency=bad",
		"deadline=bad",
		"suspend=bad",
		"foo=bar",
	}
	for _, tt := range tests {
		if _, err := parsePolicyModifier(tt); err == nil {
			t.Fatalf("expected parsePolicyModifier error for %q", tt)
		}
	}
}

func TestLoadJobDefinitionInvalidLine(t *testing.T) {
	path := writeTempKrontab(t, `
0 0 * * * @win(after,bad) name=backup command=/bin/echo
`)
	_, err := loadJobDefinition(path, "backup", explainSettings{
		Window:       time.Hour,
		Mode:         core.WindowModeAfter,
		Dist:         core.DistributionUniform,
		Timezone:     "UTC",
		SeedStrategy: core.SeedStrategyStable,
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
