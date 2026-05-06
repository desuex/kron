package daemon

import (
	"testing"

	"kron/core/pkg/core"
)

func TestParseConstraintSpecValid(t *testing.T) {
	got, err := parseConstraintSpec("hours=8-10,12;dow=MON-FRI;dom=1-5;months=JAN-MAR;between=09:00-17:30;date=2026-03-02;dates=2026-03-10..2026-03-12")
	if err != nil {
		t.Fatalf("parseConstraintSpec error: %v", err)
	}

	if len(got.hours) == 0 || len(got.dow) == 0 || len(got.dom) == 0 || len(got.months) == 0 || len(got.between) != 1 || len(got.dates) != 2 {
		t.Fatalf("unexpected parsed spec: %+v", got)
	}
}

func TestParseConstraintSpecInvalid(t *testing.T) {
	tests := []string{
		"",
		"unknown=x",
		"hours=25",
		"dow=BAD",
		"dom=0",
		"months=FOO",
		"between=18:00-09:00",
		"between=bad",
		"date=2026-13-01",
		"dates=2026-03-10..2026-03-01",
		"hours=",
	}
	for _, tt := range tests {
		if _, err := parseConstraintSpec(tt); err == nil {
			t.Fatalf("expected error for %q", tt)
		}
	}
}

func TestApplyConstraintModifier(t *testing.T) {
	var spec core.ConstraintSpec
	if err := applyConstraintModifier(&spec, "only", "hours=8-10;dow=MON;dom=1;months=MAR;date=2026-03-02"); err != nil {
		t.Fatalf("apply only error: %v", err)
	}
	if err := applyConstraintModifier(&spec, "avoid", "between=12:00-13:00;dates=2026-03-10..2026-03-12"); err != nil {
		t.Fatalf("apply avoid error: %v", err)
	}
	if len(spec.OnlyHours) == 0 || len(spec.OnlyDOW) == 0 || len(spec.OnlyDOM) == 0 || len(spec.OnlyMonths) == 0 || len(spec.OnlyDates) == 0 || len(spec.AvoidBetween) == 0 || len(spec.AvoidDates) == 0 {
		t.Fatalf("unexpected constraint spec: %+v", spec)
	}
	if err := applyConstraintModifier(&spec, "bad", "hours=1"); err == nil {
		t.Fatalf("expected unsupported modifier error")
	}
}

func TestConstraintHelperEdgeCases(t *testing.T) {
	if _, err := parseDateRangeYYYYMMDD("2026-03-10"); err == nil {
		t.Fatalf("expected date range separator error")
	}
	if _, err := parseDateRangeYYYYMMDD("2026-03-10..bad"); err == nil {
		t.Fatalf("expected date range parse error")
	}
	if _, err := parseHHMM("24:00"); err == nil {
		t.Fatalf("expected invalid hour error")
	}
	if _, err := parseBetweenRange("09:00"); err == nil {
		t.Fatalf("expected missing between separator error")
	}
	if _, err := parseConstraintIntRangeSet("1,,2", 0, 23, nil, false); err == nil {
		t.Fatalf("expected empty range element error")
	}
	if _, err := parseConstraintIntRangeSet("5-1", 0, 23, nil, false); err == nil {
		t.Fatalf("expected reversed range error")
	}
}

func TestConstraintHelperAdditionalCoverage(t *testing.T) {
	rng, err := parseBetweenRange("00:00-23:59")
	if err != nil {
		t.Fatalf("expected full-day between range parse: %v", err)
	}
	if rng.StartMinute != 0 || rng.EndMinute != 1439 {
		t.Fatalf("unexpected between range parse: %+v", rng)
	}

	if _, err := parseBetweenRange("10:00-bad"); err == nil {
		t.Fatalf("expected invalid end time error")
	}

	if v, err := parseConstraintValue("7", nil, true); err != nil || v != 0 {
		t.Fatalf("expected DOW 7 remap to 0, got value=%d err=%v", v, err)
	}
	if _, err := parseConstraintValue("bad", nil, false); err == nil {
		t.Fatalf("expected parseConstraintValue error")
	}
}
