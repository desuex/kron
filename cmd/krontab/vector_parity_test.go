package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

func TestExplainMatchesSelectedCoreVectors(t *testing.T) {
	tests := []struct {
		file string
		name string
	}{
		{file: "v1.json", name: "uniform_after_1h"},
		{file: "v3.json", name: "uniform_daily_seed_strategy"},
		{file: "v7.json", name: "skew_late_center_shape_2_5"},
		{file: "v4.json", name: "constraints_unschedulable_zero_window"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.file+"/"+tt.name, func(t *testing.T) {
			vec := loadCoreVectorCase(t, tt.file, tt.name)
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

func loadCoreVectorCase(t *testing.T, file, name string) coreVectorCase {
	t.Helper()

	path := coreVectorPath(t, file)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vector file %s: %v", path, err)
	}

	var vf coreVectorFile
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatalf("decode vector file %s: %v", path, err)
	}

	for _, c := range vf.Cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("vector case %q not found in %s", name, path)
	return coreVectorCase{}
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

func buildKrontabLineFromVector(t *testing.T, c coreVectorCase) string {
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

	if len(c.Input.OnlyHours) > 0 {
		hours := make([]string, 0, len(c.Input.OnlyHours))
		for _, h := range c.Input.OnlyHours {
			hours = append(hours, strconv.Itoa(h))
		}
		mods = append(mods, fmt.Sprintf("@only(hours=%s)", strings.Join(hours, ",")))
	}

	tokens := []string{"*", "*", "*", "*", "*"}
	tokens = append(tokens, mods...)
	tokens = append(tokens, "name="+job, "command=/bin/true")
	return strings.Join(tokens, " ")
}

func mustParseRFC3339(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse RFC3339 %q: %v", value, err)
	}
	return parsed
}
