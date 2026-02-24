package core

import (
	"testing"
	"time"
)

func TestDecideDeterministic(t *testing.T) {
	input := DecideInput{
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
	if a.ChosenTime.Before(a.WindowStart) || !a.ChosenTime.Before(a.WindowEnd) {
		t.Fatalf("chosen time outside window: %s not in [%s, %s)", a.ChosenTime, a.WindowStart, a.WindowEnd)
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

func TestDecideAllowsZeroWindow(t *testing.T) {
	period := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)
	d, err := Decide(DecideInput{
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
}
