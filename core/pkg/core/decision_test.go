package core

import (
	"errors"
	"testing"
	"time"
)

func TestDecideDeterministic(t *testing.T) {
	input := DecideInput{
		Identity:    "backup",
		Job:         "backup",
		PeriodStart: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC),
		Window:      time.Hour,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
	}

	a, err := Decide(input)
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	b, err := Decide(input)
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}

	if a.SeedHash != b.SeedHash || !a.ChosenTime.Equal(b.ChosenTime) {
		t.Fatalf("decision not deterministic")
	}
	if a.ChosenTime.Before(a.WindowStart) || a.ChosenTime.After(a.WindowEnd) {
		t.Fatalf("chosen time outside window: %s not in [%s, %s]", a.ChosenTime, a.WindowStart, a.WindowEnd)
	}
	if a.PeriodID != "2026-02-24T10:00:00Z" {
		t.Fatalf("period id mismatch: got %s", a.PeriodID)
	}
	if a.SeedStrategy != SeedStrategyStable {
		t.Fatalf("seed strategy mismatch: got %s", a.SeedStrategy)
	}
	if a.PeriodKey != a.PeriodID {
		t.Fatalf("period key mismatch: got %s want %s", a.PeriodKey, a.PeriodID)
	}
}

func TestComputeWindowModes(t *testing.T) {
	p := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)
	w := time.Hour

	s, e := computeWindow(p, WindowModeAfter, w)
	if !s.Equal(p) || !e.Equal(p.Add(w)) {
		t.Fatalf("after window mismatch")
	}

	s, e = computeWindow(p, WindowModeBefore, w)
	if !s.Equal(p.Add(-w)) || !e.Equal(p) {
		t.Fatalf("before window mismatch")
	}

	s, e = computeWindow(p, WindowModeCenter, w)
	if !s.Equal(p.Add(-30*time.Minute)) || !e.Equal(p.Add(30*time.Minute)) {
		t.Fatalf("center window mismatch")
	}
}

func TestDecideRejectsInvalidMode(t *testing.T) {
	_, err := Decide(DecideInput{
		Identity:    "backup",
		Job:         "backup",
		PeriodStart: time.Now(),
		Window:      time.Hour,
		Mode:        WindowMode("bad"),
		Dist:        DistributionUniform,
	})
	if err == nil {
		t.Fatalf("expected error for invalid mode")
	}
}

func TestDecideRejectsUnsupportedDistribution(t *testing.T) {
	_, err := Decide(DecideInput{
		Identity:    "backup",
		Job:         "backup",
		PeriodStart: time.Now(),
		Window:      time.Hour,
		Mode:        WindowModeAfter,
		Dist:        Distribution("normal"),
	})
	if err == nil {
		t.Fatalf("expected error for unsupported distribution")
	}
}

func TestDecideAllowsZeroWindow(t *testing.T) {
	period := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)
	d, err := Decide(DecideInput{
		Identity:    "backup",
		Job:         "backup",
		PeriodStart: period,
		Window:      0,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
	})
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if !d.WindowStart.Equal(period) {
		t.Fatalf("window start mismatch: got %s want %s", d.WindowStart, period)
	}
	if !d.WindowEnd.Equal(period) {
		t.Fatalf("window end mismatch: got %s want %s", d.WindowEnd, period)
	}
	if !d.ChosenTime.Equal(period) {
		t.Fatalf("chosen time mismatch: got %s want %s", d.ChosenTime, period)
	}
	if d.AttemptCount != 0 {
		t.Fatalf("expected zero attempts for zero window, got %d", d.AttemptCount)
	}
}

func TestMapDistribution(t *testing.T) {
	u := 0.25

	if got := mapDistribution(DistributionUniform, u, 2.0); got != u {
		t.Fatalf("uniform mapping mismatch: got %f want %f", got, u)
	}
	if got := mapDistribution(DistributionSkewEarly, u, 2.0); got >= u {
		t.Fatalf("skewEarly expected <= u for u in (0,1), got %f u=%f", got, u)
	}
	if got := mapDistribution(DistributionSkewLate, u, 2.0); got <= u {
		t.Fatalf("skewLate expected >= u for u in (0,1), got %f u=%f", got, u)
	}

	earlyWeak := mapDistribution(DistributionSkewEarly, u, 1.2)
	earlyStrong := mapDistribution(DistributionSkewEarly, u, 4.0)
	if earlyStrong >= earlyWeak {
		t.Fatalf("expected stronger skewEarly shape to push earlier: weak=%f strong=%f", earlyWeak, earlyStrong)
	}

	lateWeak := mapDistribution(DistributionSkewLate, u, 1.2)
	lateStrong := mapDistribution(DistributionSkewLate, u, 4.0)
	if lateStrong <= lateWeak {
		t.Fatalf("expected stronger skewLate shape to push later: weak=%f strong=%f", lateWeak, lateStrong)
	}
}

func TestDecideSkewShapeValidation(t *testing.T) {
	_, err := Decide(DecideInput{
		Identity:    "x",
		Job:         "x",
		PeriodStart: time.Now(),
		Window:      time.Second,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		SkewShape:   1.5,
	})
	if !errors.Is(err, ErrInvalidDistribution) {
		t.Fatalf("expected ErrInvalidDistribution for uniform skew shape, got %v", err)
	}

	_, err = Decide(DecideInput{
		Identity:    "x",
		Job:         "x",
		PeriodStart: time.Now(),
		Window:      time.Second,
		Mode:        WindowModeAfter,
		Dist:        DistributionSkewLate,
		SkewShape:   -1,
	})
	if !errors.Is(err, ErrInvalidDistribution) {
		t.Fatalf("expected ErrInvalidDistribution for negative skew shape, got %v", err)
	}
}

func TestDecideSkewShapeAffectsDecision(t *testing.T) {
	base := DecideInput{
		Identity:    "shape/test",
		Job:         "shape/test",
		PeriodStart: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC),
		Window:      90 * time.Minute,
		Mode:        WindowModeCenter,
		Dist:        DistributionSkewLate,
	}

	weak, err := Decide(base)
	if err != nil {
		t.Fatalf("Decide weak shape error: %v", err)
	}

	strongInput := base
	strongInput.SkewShape = 4.0
	strong, err := Decide(strongInput)
	if err != nil {
		t.Fatalf("Decide strong shape error: %v", err)
	}

	if !strong.ChosenTime.After(weak.ChosenTime) {
		t.Fatalf("expected stronger skew shape to choose later time: weak=%s strong=%s", weak.ChosenTime, strong.ChosenTime)
	}
}

func TestDecideConstraintsAndUnschedulable(t *testing.T) {
	period := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)

	okDecision, err := Decide(DecideInput{
		Identity:    "constraint/ok",
		Job:         "constraint/ok",
		PeriodStart: period,
		Window:      0,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{OnlyHours: []int{10}},
	})
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if okDecision.Unschedulable {
		t.Fatalf("expected schedulable decision")
	}
	if !okDecision.ChosenTime.Equal(period) {
		t.Fatalf("chosen mismatch: got %s want %s", okDecision.ChosenTime, period)
	}

	unsched, err := Decide(DecideInput{
		Identity:    "constraint/unsched",
		Job:         "constraint/unsched",
		PeriodStart: period,
		Window:      0,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{OnlyHours: []int{11}},
	})
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if !unsched.Unschedulable {
		t.Fatalf("expected unschedulable decision")
	}
	if !unsched.ChosenTime.IsZero() {
		t.Fatalf("expected zero chosen time when unschedulable")
	}
	if unsched.Reason == "" {
		t.Fatalf("expected unschedulable reason")
	}
}

func TestDecideConstraintValidation(t *testing.T) {
	_, err := Decide(DecideInput{
		Identity:    "constraint/invalid",
		Job:         "constraint/invalid",
		PeriodStart: time.Now(),
		Window:      time.Hour,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{OnlyHours: []int{-1}},
	})
	if err == nil {
		t.Fatalf("expected invalid constraint error")
	}

	_, err = Decide(DecideInput{
		Identity:    "constraint/invalid-dow",
		Job:         "constraint/invalid-dow",
		PeriodStart: time.Now(),
		Window:      time.Hour,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{OnlyDOW: []int{8}},
	})
	if err == nil {
		t.Fatalf("expected invalid dow constraint error")
	}

	_, err = Decide(DecideInput{
		Identity:    "constraint/invalid-between",
		Job:         "constraint/invalid-between",
		PeriodStart: time.Now(),
		Window:      time.Hour,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{OnlyBetween: []TimeRange{{StartMinute: 600, EndMinute: 500}}},
	})
	if err == nil {
		t.Fatalf("expected invalid between constraint error")
	}

	_, err = Decide(DecideInput{
		Identity:    "constraint/invalid-dom",
		Job:         "constraint/invalid-dom",
		PeriodStart: time.Now(),
		Window:      time.Hour,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{OnlyDOM: []int{0}},
	})
	if err == nil {
		t.Fatalf("expected invalid dom constraint error")
	}

	_, err = Decide(DecideInput{
		Identity:    "constraint/invalid-month",
		Job:         "constraint/invalid-month",
		PeriodStart: time.Now(),
		Window:      time.Hour,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{OnlyMonths: []int{13}},
	})
	if err == nil {
		t.Fatalf("expected invalid months constraint error")
	}

	_, err = Decide(DecideInput{
		Identity:    "constraint/invalid-dates",
		Job:         "constraint/invalid-dates",
		PeriodStart: time.Now(),
		Window:      time.Hour,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{OnlyDates: []DateRange{{StartDay: 20260305, EndDay: 20260301}}},
	})
	if err == nil {
		t.Fatalf("expected invalid dates constraint error")
	}
}

func TestDecideTypedErrors(t *testing.T) {
	_, err := Decide(DecideInput{})
	if !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("expected ErrInvalidIdentity, got %v", err)
	}

	_, err = Decide(DecideInput{
		Identity:    "x",
		Job:         "x",
		PeriodStart: time.Now(),
		Window:      -1,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
	})
	if !errors.Is(err, ErrInvalidWindow) {
		t.Fatalf("expected ErrInvalidWindow, got %v", err)
	}

	_, err = Decide(DecideInput{
		Identity:    "x",
		Job:         "x",
		PeriodStart: time.Now(),
		Window:      time.Second,
		Mode:        WindowMode("bad"),
		Dist:        DistributionUniform,
	})
	if !errors.Is(err, ErrInvalidWindowMode) {
		t.Fatalf("expected ErrInvalidWindowMode, got %v", err)
	}

	_, err = Decide(DecideInput{
		Identity:    "x",
		Job:         "x",
		PeriodStart: time.Now(),
		Window:      time.Second,
		Mode:        WindowModeAfter,
		Dist:        Distribution("normal"),
	})
	if !errors.Is(err, ErrInvalidDistribution) {
		t.Fatalf("expected ErrInvalidDistribution, got %v", err)
	}

	_, err = Decide(DecideInput{
		Identity:     "x",
		Job:          "x",
		PeriodStart:  time.Now(),
		Window:       time.Second,
		Mode:         WindowModeAfter,
		Dist:         DistributionUniform,
		SeedStrategy: SeedStrategy("bad"),
	})
	if !errors.Is(err, ErrInvalidSeedStrategy) {
		t.Fatalf("expected ErrInvalidSeedStrategy, got %v", err)
	}

	_, err = Decide(DecideInput{
		Identity:    "x",
		Job:         "x",
		PeriodStart: time.Now(),
		Timezone:    "No/Such_TZ",
		Window:      time.Second,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
	})
	if !errors.Is(err, ErrInvalidTimezone) {
		t.Fatalf("expected ErrInvalidTimezone, got %v", err)
	}

	_, err = Decide(DecideInput{
		Identity:    "x",
		Job:         "x",
		PeriodStart: time.Now(),
		Window:      time.Second,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{OnlyHours: []int{-1}},
	})
	if !errors.Is(err, ErrInvalidConstraint) {
		t.Fatalf("expected ErrInvalidConstraint, got %v", err)
	}

	_, err = Decide(DecideInput{
		Identity:    "x",
		Job:         "x",
		PeriodStart: time.Now(),
		Window:      time.Second,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{OnlyDOW: []int{9}},
	})
	if !errors.Is(err, ErrInvalidConstraint) {
		t.Fatalf("expected ErrInvalidConstraint for dow, got %v", err)
	}

	_, err = Decide(DecideInput{
		Identity:    "x",
		Job:         "x",
		PeriodStart: time.Now(),
		Window:      time.Second,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{OnlyDOM: []int{0}},
	})
	if !errors.Is(err, ErrInvalidConstraint) {
		t.Fatalf("expected ErrInvalidConstraint for dom, got %v", err)
	}
}

func TestDecideConstraintsDOWAndBetween(t *testing.T) {
	// 2026-03-02 is Monday.
	period := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)

	ok, err := Decide(DecideInput{
		Identity:    "constraint/dow-between-ok",
		Job:         "constraint/dow-between-ok",
		PeriodStart: period,
		Window:      0,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{
			OnlyDOW:     []int{1}, // Monday
			OnlyBetween: []TimeRange{{StartMinute: 12 * 60, EndMinute: 12*60 + 30}},
		},
	})
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if ok.Unschedulable {
		t.Fatalf("expected schedulable decision")
	}

	blocked, err := Decide(DecideInput{
		Identity:    "constraint/dow-between-blocked",
		Job:         "constraint/dow-between-blocked",
		PeriodStart: period,
		Window:      0,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{
			OnlyDOW:      []int{1},
			AvoidBetween: []TimeRange{{StartMinute: 12 * 60, EndMinute: 12 * 60}},
		},
	})
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if !blocked.Unschedulable {
		t.Fatalf("expected unschedulable due to avoid-between")
	}
}

func TestDecideConstraintsDOMMonthsAndDates(t *testing.T) {
	period := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC) // Monday, dom=2, month=3

	ok, err := Decide(DecideInput{
		Identity:    "constraint/dom-month-date-ok",
		Job:         "constraint/dom-month-date-ok",
		PeriodStart: period,
		Window:      0,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{
			OnlyDOM:    []int{2},
			OnlyMonths: []int{3},
			OnlyDates:  []DateRange{{StartDay: 20260301, EndDay: 20260305}},
		},
	})
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if ok.Unschedulable {
		t.Fatalf("expected schedulable decision")
	}

	blocked, err := Decide(DecideInput{
		Identity:    "constraint/dom-month-date-blocked",
		Job:         "constraint/dom-month-date-blocked",
		PeriodStart: period,
		Window:      0,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
		Constraints: ConstraintSpec{
			OnlyDOM:    []int{2},
			OnlyMonths: []int{3},
			AvoidDates: []DateRange{{StartDay: 20260302, EndDay: 20260302}},
		},
	})
	if err != nil {
		t.Fatalf("Decide error: %v", err)
	}
	if !blocked.Unschedulable {
		t.Fatalf("expected unschedulable due to avoid-dates")
	}
}

func TestClampUnitBoundaries(t *testing.T) {
	if got := clampUnit(-0.5); got != 0 {
		t.Fatalf("expected clamp below zero to 0, got %f", got)
	}
	if got := clampUnit(0.5); got != 0.5 {
		t.Fatalf("expected clamp in-range unchanged, got %f", got)
	}
	if got := clampUnit(1.0); !(got < 1.0) {
		t.Fatalf("expected clamp at/above one to be <1, got %f", got)
	}
}

func TestValidateConstraintsAndCandidateAllowedHelpers(t *testing.T) {
	rt, err := validateConstraints(ConstraintSpec{
		OnlyHours:   []int{8, 9},
		AvoidHours:  []int{10},
		OnlyDOW:     []int{1}, // Monday
		AvoidDOW:    []int{2}, // Tuesday
		OnlyDOM:     []int{2},
		AvoidDOM:    []int{3},
		OnlyMonths:  []int{3},
		AvoidMonths: []int{4},
		OnlyBetween: []TimeRange{{StartMinute: 8 * 60, EndMinute: 9*60 + 59}},
		AvoidBetween: []TimeRange{
			{StartMinute: 8*60 + 30, EndMinute: 8*60 + 35},
		},
		OnlyDates:  []DateRange{{StartDay: 20260301, EndDay: 20260305}},
		AvoidDates: []DateRange{{StartDay: 20260303, EndDay: 20260303}},
	})
	if err != nil {
		t.Fatalf("validateConstraints error: %v", err)
	}

	mon0820 := time.Date(2026, 3, 2, 8, 20, 0, 0, time.UTC) // Monday
	if !candidateAllowed(mon0820, time.UTC, rt) {
		t.Fatalf("expected candidate to be allowed")
	}

	mon0832 := time.Date(2026, 3, 2, 8, 32, 0, 0, time.UTC)
	if candidateAllowed(mon0832, time.UTC, rt) {
		t.Fatalf("expected candidate blocked by avoid-between")
	}

	mon1030 := time.Date(2026, 3, 2, 10, 30, 0, 0, time.UTC)
	if candidateAllowed(mon1030, time.UTC, rt) {
		t.Fatalf("expected candidate blocked by only-hours/avoid-hours")
	}

	tue0820 := time.Date(2026, 3, 3, 8, 20, 0, 0, time.UTC) // Tuesday
	if candidateAllowed(tue0820, time.UTC, rt) {
		t.Fatalf("expected candidate blocked by only-dow/avoid-dow")
	}

	wed0820 := time.Date(2026, 3, 4, 8, 20, 0, 0, time.UTC) // Wednesday, dow mismatch
	if candidateAllowed(wed0820, time.UTC, rt) {
		t.Fatalf("expected candidate blocked by only-dow")
	}

	fri0820 := time.Date(2026, 3, 6, 8, 20, 0, 0, time.UTC) // Friday, day not in only date range
	if candidateAllowed(fri0820, time.UTC, rt) {
		t.Fatalf("expected candidate blocked by only-dates")
	}
}

func TestDecidePeriodKeyStrategies(t *testing.T) {
	period := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	daily, err := Decide(DecideInput{
		Identity:     "daily/test",
		Job:          "daily/test",
		PeriodStart:  period,
		Timezone:     "UTC",
		Window:       time.Hour,
		Mode:         WindowModeAfter,
		Dist:         DistributionUniform,
		SeedStrategy: SeedStrategyDaily,
	})
	if err != nil {
		t.Fatalf("daily decide error: %v", err)
	}
	if daily.PeriodKey != "2026-03-01" {
		t.Fatalf("daily period key mismatch: got %s", daily.PeriodKey)
	}

	weekly, err := Decide(DecideInput{
		Identity:     "weekly/test",
		Job:          "weekly/test",
		PeriodStart:  period,
		Timezone:     "UTC",
		Window:       time.Hour,
		Mode:         WindowModeAfter,
		Dist:         DistributionUniform,
		SeedStrategy: SeedStrategyWeekly,
	})
	if err != nil {
		t.Fatalf("weekly decide error: %v", err)
	}
	if weekly.PeriodKey != "2026-W09" {
		t.Fatalf("weekly period key mismatch: got %s", weekly.PeriodKey)
	}
}

func TestDecideAroundAliasAndValidation(t *testing.T) {
	period := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)

	d, err := Decide(DecideInput{
		Identity:    "backup",
		Job:         "backup",
		PeriodStart: period,
		Window:      time.Hour,
		Mode:        WindowModeAround,
		Dist:        DistributionUniform,
	})
	if err != nil {
		t.Fatalf("decide error: %v", err)
	}
	if d.Mode != WindowModeCenter {
		t.Fatalf("expected around alias to normalize to center, got %s", d.Mode)
	}

	_, err = Decide(DecideInput{
		Identity:     "backup",
		Job:          "backup",
		PeriodStart:  period,
		Window:       time.Hour,
		Mode:         WindowModeAfter,
		Dist:         DistributionUniform,
		SeedStrategy: SeedStrategy("bad"),
	})
	if err == nil {
		t.Fatalf("expected invalid seed strategy error")
	}

	_, err = Decide(DecideInput{
		Identity:    "backup",
		Job:         "backup",
		PeriodStart: period,
		Timezone:    "No/Such_TZ",
		Window:      time.Hour,
		Mode:        WindowModeAfter,
		Dist:        DistributionUniform,
	})
	if err == nil {
		t.Fatalf("expected invalid timezone error")
	}
}
