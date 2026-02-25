package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

type vectorFile struct {
	Version string       `json:"version"`
	Cases   []vectorCase `json:"cases"`
}

type vectorCase struct {
	Name     string      `json:"name"`
	Input    vectorInput `json:"input"`
	Expected vectorOut   `json:"expected"`
}

type vectorInput struct {
	Identity     string          `json:"identity"`
	Job          string          `json:"job"`
	PeriodStart  string          `json:"periodStart"`
	Timezone     string          `json:"timezone"`
	Window       string          `json:"window"`
	Mode         string          `json:"mode"`
	Distribution string          `json:"distribution"`
	SkewShape    float64         `json:"skewShape,omitempty"`
	SeedStrategy string          `json:"seedStrategy"`
	Salt         string          `json:"salt"`
	OnlyHours    []int           `json:"onlyHours,omitempty"`
	AvoidHours   []int           `json:"avoidHours,omitempty"`
	OnlyDOW      []int           `json:"onlyDow,omitempty"`
	AvoidDOW     []int           `json:"avoidDow,omitempty"`
	OnlyDOM      []int           `json:"onlyDom,omitempty"`
	AvoidDOM     []int           `json:"avoidDom,omitempty"`
	OnlyMonths   []int           `json:"onlyMonths,omitempty"`
	AvoidMonths  []int           `json:"avoidMonths,omitempty"`
	OnlyBetween  []TimeRangeJSON `json:"onlyBetween,omitempty"`
	AvoidBetween []TimeRangeJSON `json:"avoidBetween,omitempty"`
	OnlyDates    []DateRangeJSON `json:"onlyDates,omitempty"`
	AvoidDates   []DateRangeJSON `json:"avoidDates,omitempty"`
}

type TimeRangeJSON struct {
	StartMinute int `json:"startMinute"`
	EndMinute   int `json:"endMinute"`
}

type DateRangeJSON struct {
	StartDay int `json:"startDay"`
	EndDay   int `json:"endDay"`
}

type vectorOut struct {
	PeriodID      string `json:"periodId"`
	PeriodKey     string `json:"periodKey"`
	WindowStart   string `json:"windowStart"`
	WindowEnd     string `json:"windowEnd"`
	SeedHash      string `json:"seedHash"`
	ChosenTime    string `json:"chosenTime"`
	Attempts      *int   `json:"attempts,omitempty"`
	Unschedulable *bool  `json:"unschedulable,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

func TestGoldenVectors(t *testing.T) { // NOSONAR
	matches, err := filepath.Glob("../../testdata/vectors/*.json")
	if err != nil {
		t.Fatalf("glob vectors: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no vector files found")
	}
	sort.Strings(matches)

	for _, path := range matches {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			vf := readVectorFile(t, path)
			if len(vf.Cases) == 0 {
				t.Fatalf("vector file has no cases: %s", path)
			}
			for _, tc := range vf.Cases {
				tc := tc
				t.Run(tc.Name, func(t *testing.T) {
					input := mustVectorInput(t, tc.Input)
					got, err := Decide(input)
					if err != nil {
						t.Fatalf("Decide error: %v", err)
					}

					wantStart := mustRFC3339(t, tc.Expected.WindowStart)
					wantEnd := mustRFC3339(t, tc.Expected.WindowEnd)
					var wantChosen time.Time
					if tc.Expected.ChosenTime != "" {
						wantChosen = mustRFC3339(t, tc.Expected.ChosenTime)
					}

					if !got.WindowStart.Equal(wantStart) {
						t.Fatalf("windowStart mismatch: got=%s want=%s", got.WindowStart.Format(time.RFC3339), tc.Expected.WindowStart)
					}
					if !got.WindowEnd.Equal(wantEnd) {
						t.Fatalf("windowEnd mismatch: got=%s want=%s", got.WindowEnd.Format(time.RFC3339), tc.Expected.WindowEnd)
					}
					if tc.Expected.PeriodID != "" && got.PeriodID != tc.Expected.PeriodID {
						t.Fatalf("periodID mismatch: got=%s want=%s", got.PeriodID, tc.Expected.PeriodID)
					}
					if tc.Expected.PeriodKey != "" && got.PeriodKey != tc.Expected.PeriodKey {
						t.Fatalf("periodKey mismatch: got=%s want=%s", got.PeriodKey, tc.Expected.PeriodKey)
					}
					if got.SeedHash != tc.Expected.SeedHash {
						t.Fatalf("seedHash mismatch: got=%s want=%s", got.SeedHash, tc.Expected.SeedHash)
					}
					if tc.Expected.ChosenTime == "" {
						if !got.ChosenTime.IsZero() {
							t.Fatalf("expected zero chosenTime, got=%s", got.ChosenTime.Format(time.RFC3339))
						}
					} else if !got.ChosenTime.Equal(wantChosen) {
						t.Fatalf("chosenTime mismatch: got=%s want=%s", got.ChosenTime.Format(time.RFC3339), tc.Expected.ChosenTime)
					}
					if tc.Expected.Attempts != nil && got.AttemptCount != *tc.Expected.Attempts {
						t.Fatalf("attempt count mismatch: got=%d want=%d", got.AttemptCount, *tc.Expected.Attempts)
					}
					if tc.Expected.Unschedulable != nil && got.Unschedulable != *tc.Expected.Unschedulable {
						t.Fatalf("unschedulable mismatch: got=%v want=%v", got.Unschedulable, *tc.Expected.Unschedulable)
					}
					if tc.Expected.Reason != "" && got.Reason != tc.Expected.Reason {
						t.Fatalf("reason mismatch: got=%q want=%q", got.Reason, tc.Expected.Reason)
					}
				})
			}
		})
	}
}

func readVectorFile(t *testing.T, path string) vectorFile {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read vector file: %v", err)
	}
	var vf vectorFile
	if err := json.Unmarshal(b, &vf); err != nil {
		t.Fatalf("parse vector file: %v", err)
	}
	return vf
}

func mustVectorInput(t *testing.T, in vectorInput) DecideInput {
	t.Helper()

	periodStart := mustRFC3339(t, in.PeriodStart)
	window, err := time.ParseDuration(in.Window)
	if err != nil {
		t.Fatalf("parse window duration %q: %v", in.Window, err)
	}

	return DecideInput{
		Identity:     in.Identity,
		Job:          in.Job,
		PeriodStart:  periodStart,
		Timezone:     in.Timezone,
		Window:       window,
		Mode:         WindowMode(in.Mode),
		Dist:         Distribution(in.Distribution),
		SkewShape:    in.SkewShape,
		SeedStrategy: SeedStrategy(in.SeedStrategy),
		Salt:         in.Salt,
		Constraints: ConstraintSpec{
			OnlyHours:    in.OnlyHours,
			AvoidHours:   in.AvoidHours,
			OnlyDOW:      in.OnlyDOW,
			AvoidDOW:     in.AvoidDOW,
			OnlyDOM:      in.OnlyDOM,
			AvoidDOM:     in.AvoidDOM,
			OnlyMonths:   in.OnlyMonths,
			AvoidMonths:  in.AvoidMonths,
			OnlyBetween:  toCoreTimeRanges(in.OnlyBetween),
			AvoidBetween: toCoreTimeRanges(in.AvoidBetween),
			OnlyDates:    toCoreDateRanges(in.OnlyDates),
			AvoidDates:   toCoreDateRanges(in.AvoidDates),
		},
	}
}

func toCoreTimeRanges(in []TimeRangeJSON) []TimeRange {
	out := make([]TimeRange, 0, len(in))
	for _, r := range in {
		out = append(out, TimeRange{StartMinute: r.StartMinute, EndMinute: r.EndMinute})
	}
	return out
}

func toCoreDateRanges(in []DateRangeJSON) []DateRange {
	out := make([]DateRange, 0, len(in))
	for _, r := range in {
		out = append(out, DateRange{StartDay: r.StartDay, EndDay: r.EndDay})
	}
	return out
}

func mustRFC3339(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse RFC3339 %q: %v", s, err)
	}
	return v.UTC()
}
