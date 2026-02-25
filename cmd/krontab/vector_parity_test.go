package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"kron/core/pkg/core"
)

type coreVectorFile struct {
	Cases []coreVectorCase `json:"cases"`
}

type coreVectorCase struct {
	Name     string             `json:"name"`
	Input    coreVectorInput    `json:"input"`
	Expected coreVectorExpected `json:"expected"`
}

type coreVectorInput struct {
	Identity     string  `json:"identity"`
	Job          string  `json:"job"`
	PeriodStart  string  `json:"periodStart"`
	Timezone     string  `json:"timezone"`
	Window       string  `json:"window"`
	Mode         string  `json:"mode"`
	Distribution string  `json:"distribution"`
	SkewShape    float64 `json:"skewShape"`
	SeedStrategy string  `json:"seedStrategy"`
	Salt         string  `json:"salt"`
	OnlyHours    []int   `json:"onlyHours"`
	AvoidHours   []int   `json:"avoidHours"`
	OnlyDOW      []int   `json:"onlyDow"`
	AvoidDOW     []int   `json:"avoidDow"`
	OnlyDOM      []int   `json:"onlyDom"`
	AvoidDOM     []int   `json:"avoidDom"`
	OnlyMonths   []int   `json:"onlyMonths"`
	AvoidMonths  []int   `json:"avoidMonths"`
	OnlyBetween  []vectorTimeRange
	AvoidBetween []vectorTimeRange
	OnlyDates    []vectorDateRange
	AvoidDates   []vectorDateRange
}

type coreVectorExpected struct {
	PeriodID      string `json:"periodId"`
	PeriodKey     string `json:"periodKey"`
	WindowStart   string `json:"windowStart"`
	WindowEnd     string `json:"windowEnd"`
	SeedHash      string `json:"seedHash"`
	ChosenTime    string `json:"chosenTime"`
	Attempts      int    `json:"attempts"`
	Unschedulable bool   `json:"unschedulable"`
	Reason        string `json:"reason"`
}

type vectorTimeRange struct {
	StartMinute int `json:"startMinute"`
	EndMinute   int `json:"endMinute"`
}

type vectorDateRange struct {
	StartDay int `json:"startDay"`
	EndDay   int `json:"endDay"`
}

type namedCoreVectorCase struct {
	file string
	c    coreVectorCase
}

func TestExplainMatchesSupportedCoreVectors(t *testing.T) { // NOSONAR
	cases := loadAllCoreVectorCases(t)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.file+"/"+tc.c.Name, func(t *testing.T) {
			vec := tc.c
			cfg := writeTempKrontab(t, buildKrontabLineFromVector(t, vec))

			stdout, _ := captureOutput(t, func() {
				err := runExplain([]string{
					vec.Input.Job,
					"--file", cfg,
					"--at", vec.Input.PeriodStart,
					"--format", "json",
				})
				if err != nil {
					t.Fatalf("runExplain error: %v", err)
				}
			})

			var got core.Decision
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("json parse error: %v", err)
			}

			if got.PeriodID != vec.Expected.PeriodID {
				t.Fatalf("periodId mismatch: got %q want %q", got.PeriodID, vec.Expected.PeriodID)
			}
			if got.PeriodKey != vec.Expected.PeriodKey {
				t.Fatalf("periodKey mismatch: got %q want %q", got.PeriodKey, vec.Expected.PeriodKey)
			}
			if !got.WindowStart.Equal(mustParseRFC3339(t, vec.Expected.WindowStart)) {
				t.Fatalf("windowStart mismatch: got %s want %s", got.WindowStart, vec.Expected.WindowStart)
			}
			if !got.WindowEnd.Equal(mustParseRFC3339(t, vec.Expected.WindowEnd)) {
				t.Fatalf("windowEnd mismatch: got %s want %s", got.WindowEnd, vec.Expected.WindowEnd)
			}
			if got.SeedHash != vec.Expected.SeedHash {
				t.Fatalf("seedHash mismatch: got %q want %q", got.SeedHash, vec.Expected.SeedHash)
			}
			if got.AttemptCount != vec.Expected.Attempts {
				t.Fatalf("attemptCount mismatch: got %d want %d", got.AttemptCount, vec.Expected.Attempts)
			}
			if got.Unschedulable != vec.Expected.Unschedulable {
				t.Fatalf("unschedulable mismatch: got %v want %v", got.Unschedulable, vec.Expected.Unschedulable)
			}
			if vec.Expected.ChosenTime == "" {
				if !got.ChosenTime.IsZero() {
					t.Fatalf("expected zero chosenTime, got %s", got.ChosenTime)
				}
			} else if !got.ChosenTime.Equal(mustParseRFC3339(t, vec.Expected.ChosenTime)) {
				t.Fatalf("chosenTime mismatch: got %s want %s", got.ChosenTime, vec.Expected.ChosenTime)
			}
			if vec.Expected.Reason != "" && got.Reason != vec.Expected.Reason {
				t.Fatalf("reason mismatch: got %q want %q", got.Reason, vec.Expected.Reason)
			}
		})
	}
}

func loadAllCoreVectorCases(t *testing.T) []namedCoreVectorCase {
	t.Helper()

	matches, err := filepath.Glob(coreVectorPath(t, "*.json"))
	if err != nil {
		t.Fatalf("glob vector files: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no vector files found")
	}
	sort.Strings(matches)

	out := make([]namedCoreVectorCase, 0)
	for _, path := range matches {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read vector file %s: %v", path, err)
		}
		var vf coreVectorFile
		if err := json.Unmarshal(raw, &vf); err != nil {
			t.Fatalf("decode vector file %s: %v", path, err)
		}
		file := filepath.Base(path)
		for _, c := range vf.Cases {
			out = append(out, namedCoreVectorCase{file: file, c: c})
		}
	}
	return out
}

func coreVectorPath(t *testing.T, file string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	base := filepath.Dir(thisFile)
	return filepath.Join(base, "..", "..", "core", "testdata", "vectors", file)
}

func buildKrontabLineFromVector(t *testing.T, c coreVectorCase) string { // NOSONAR
	t.Helper()

	job := strings.TrimSpace(c.Input.Job)
	if job == "" {
		job = strings.TrimSpace(c.Input.Identity)
	}
	if job == "" {
		t.Fatalf("vector %q has empty job/identity", c.Name)
	}

	mods := make([]string, 0, 8)

	if tz := strings.TrimSpace(c.Input.Timezone); tz != "" {
		mods = append(mods, fmt.Sprintf("@tz(%s)", tz))
	}

	mode := strings.TrimSpace(c.Input.Mode)
	if mode == "" {
		mode = "after"
	}
	if mode == "center" {
		mode = "around"
	}
	window := strings.TrimSpace(c.Input.Window)
	if window == "" {
		window = "0s"
	}
	mods = append(mods, fmt.Sprintf("@win(%s,%s)", mode, window))

	dist := strings.TrimSpace(c.Input.Distribution)
	if dist == "" {
		dist = "uniform"
	}
	switch dist {
	case "uniform":
		mods = append(mods, "@dist(uniform)")
	case "skewEarly", "skewLate":
		if c.Input.SkewShape > 0 {
			mods = append(mods, fmt.Sprintf("@dist(%s,shape=%s)", dist, strconv.FormatFloat(c.Input.SkewShape, 'f', -1, 64)))
		} else {
			mods = append(mods, fmt.Sprintf("@dist(%s)", dist))
		}
	default:
		t.Fatalf("vector %q distribution %q is not supported in MVP explain", c.Name, dist)
	}

	seedStrategy := strings.TrimSpace(c.Input.SeedStrategy)
	if seedStrategy == "" {
		seedStrategy = "stable"
	}
	if strings.TrimSpace(c.Input.Salt) != "" {
		mods = append(mods, fmt.Sprintf("@seed(%s,salt=%s)", seedStrategy, c.Input.Salt))
	} else {
		mods = append(mods, fmt.Sprintf("@seed(%s)", seedStrategy))
	}

	if onlyBody := constraintBodyFromVector(
		c.Input.OnlyHours,
		c.Input.OnlyDOW,
		c.Input.OnlyDOM,
		c.Input.OnlyMonths,
		c.Input.OnlyBetween,
		c.Input.OnlyDates,
	); onlyBody != "" {
		mods = append(mods, "@only("+onlyBody+")")
	}
	if avoidBody := constraintBodyFromVector(
		c.Input.AvoidHours,
		c.Input.AvoidDOW,
		c.Input.AvoidDOM,
		c.Input.AvoidMonths,
		c.Input.AvoidBetween,
		c.Input.AvoidDates,
	); avoidBody != "" {
		mods = append(mods, "@avoid("+avoidBody+")")
	}

	tokens := []string{"*", "*", "*", "*", "*"}
	tokens = append(tokens, mods...)
	tokens = append(tokens, "name="+job, "command=/bin/true")
	return strings.Join(tokens, " ")
}

func constraintBodyFromVector(hours, dow, dom, months []int, between []vectorTimeRange, dates []vectorDateRange) string {
	parts := make([]string, 0, 8)

	if len(hours) > 0 {
		parts = append(parts, "hours="+joinInts(hours))
	}
	if len(dow) > 0 {
		parts = append(parts, "dow="+joinInts(dow))
	}
	if len(dom) > 0 {
		parts = append(parts, "dom="+joinInts(dom))
	}
	if len(months) > 0 {
		parts = append(parts, "months="+joinInts(months))
	}
	for _, r := range between {
		parts = append(parts, fmt.Sprintf("between=%s-%s", hhmm(r.StartMinute), hhmm(r.EndMinute)))
	}
	for _, r := range dates {
		if r.StartDay == r.EndDay {
			parts = append(parts, "date="+yyyymmdd(r.StartDay))
			continue
		}
		parts = append(parts, fmt.Sprintf("dates=%s..%s", yyyymmdd(r.StartDay), yyyymmdd(r.EndDay)))
	}

	return strings.Join(parts, ";")
}

func joinInts(v []int) string {
	out := make([]string, 0, len(v))
	for _, n := range v {
		out = append(out, strconv.Itoa(n))
	}
	return strings.Join(out, ",")
}

func hhmm(totalMinutes int) string {
	h := totalMinutes / 60
	m := totalMinutes % 60
	return fmt.Sprintf("%02d:%02d", h, m)
}

func yyyymmdd(day int) string {
	y := day / 10000
	mon := (day / 100) % 100
	d := day % 100
	return fmt.Sprintf("%04d-%02d-%02d", y, mon, d)
}

func mustParseRFC3339(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse RFC3339 %q: %v", value, err)
	}
	return parsed
}
