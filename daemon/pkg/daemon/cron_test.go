package daemon

import (
	"strings"
	"testing"
	"time"
)

const errParseCronSpec = "parseCronSpec error: %v"

func TestCronNextAfterStep(t *testing.T) {
	spec, err := parseCronSpec([5]string{"*/15", "*", "*", "*", "*"}, "UTC")
	if err != nil {
		t.Fatalf(errParseCronSpec, err)
	}

	start := time.Date(2026, 2, 24, 10, 7, 30, 0, time.UTC)
	next, err := spec.NextAfter(start)
	if err != nil {
		t.Fatalf("NextAfter error: %v", err)
	}

	want := time.Date(2026, 2, 24, 10, 15, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next mismatch: got %s want %s", next, want)
	}
}

func TestCronNextN(t *testing.T) {
	spec, err := parseCronSpec([5]string{"0", "0", "*", "*", "*"}, "UTC")
	if err != nil {
		t.Fatalf(errParseCronSpec, err)
	}

	start := time.Date(2026, 2, 24, 10, 7, 0, 0, time.UTC)
	got, err := spec.NextN(start, 2)
	if err != nil {
		t.Fatalf("NextN error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("count mismatch: got %d want %d", len(got), 2)
	}

	want1 := time.Date(2026, 2, 25, 0, 0, 0, 0, time.UTC)
	want2 := time.Date(2026, 2, 26, 0, 0, 0, 0, time.UTC)
	if !got[0].Equal(want1) || !got[1].Equal(want2) {
		t.Fatalf("unexpected next values: got %v", got)
	}
}

func TestCronWithTimezone(t *testing.T) {
	spec, err := parseCronSpec([5]string{"0", "9", "*", "*", "*"}, "America/New_York")
	if err != nil {
		t.Fatalf(errParseCronSpec, err)
	}

	anchor := time.Date(2026, 2, 24, 13, 0, 0, 0, time.UTC) // 08:00 EST
	next, err := spec.NextAfter(anchor)
	if err != nil {
		t.Fatalf("NextAfter error: %v", err)
	}

	want := time.Date(2026, 2, 24, 14, 0, 0, 0, time.UTC) // 09:00 EST
	if !next.Equal(want) {
		t.Fatalf("next mismatch: got %s want %s", next, want)
	}
}

func TestParseCronSpecInvalid(t *testing.T) {
	if _, err := parseCronSpec([5]string{"bad", "*", "*", "*", "*"}, "UTC"); err == nil {
		t.Fatalf("expected minute parse error")
	}
	if _, err := parseCronSpec([5]string{"0", "0", "*", "*", "*"}, "No/Such_TZ"); err == nil {
		t.Fatalf("expected timezone error")
	}
}

func TestParseFieldAndValuePaths(t *testing.T) {
	if _, _, err := parseField("1-5/2", 0, 59, nil, false); err != nil {
		t.Fatalf("expected valid stepped range: %v", err)
	}
	if _, _, err := parseField("*/0", 0, 59, nil, false); err == nil {
		t.Fatalf("expected invalid step error")
	}
	if _, _, err := parseField("8", 0, 6, nil, true); err == nil {
		t.Fatalf("expected out-of-range error")
	}
	if _, _, err := parseField("MON", 0, 6, dowAliases, true); err != nil {
		t.Fatalf("expected alias value success: %v", err)
	}
	if _, err := parseCronValue("7", nil, true); err != nil {
		t.Fatalf("expected 7->0 mapping for dow: %v", err)
	}
	if _, err := parseCronValue("NOPE", nil, false); err == nil {
		t.Fatalf("expected parse value error")
	}
}

func TestCronMatchesModes(t *testing.T) {
	ts := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC) // Tuesday

	c := CronSpec{
		location: time.UTC,
		domAny:   true,
		dowAny:   true,
	}
	c.months[2] = true
	c.hours[10] = true
	c.minutes[0] = true
	c.dom[24] = true
	c.dow[int(time.Tuesday)] = true
	if !c.matches(ts) {
		t.Fatalf("expected match when domAny and dowAny are true")
	}

	c.domAny = true
	c.dowAny = false
	c.dow[int(time.Tuesday)] = false
	if c.matches(ts) {
		t.Fatalf("expected no match when domAny and dow doesn't match")
	}
	c.dow[int(time.Tuesday)] = true
	if !c.matches(ts) {
		t.Fatalf("expected match when domAny and dow matches")
	}

	c.domAny = false
	c.dowAny = true
	c.dom[24] = false
	if c.matches(ts) {
		t.Fatalf("expected no match when dowAny and dom doesn't match")
	}
	c.dom[24] = true
	if !c.matches(ts) {
		t.Fatalf("expected match when dowAny and dom matches")
	}
}

func TestNextNRejectsNonPositiveCount(t *testing.T) {
	spec, err := parseCronSpec([5]string{"*", "*", "*", "*", "*"}, "UTC")
	if err != nil {
		t.Fatalf(errParseCronSpec, err)
	}
	if _, err := spec.NextN(time.Now(), 0); err == nil {
		t.Fatalf("expected count error")
	}
}

func TestCronNextAfterNoMatchWithinTenYears(t *testing.T) {
	spec, err := parseCronSpec([5]string{"0", "0", "31", "2", "*"}, "UTC")
	if err != nil {
		t.Fatalf(errParseCronSpec, err)
	}

	_, err = spec.NextAfter(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatalf("expected no-match error")
	}
	if !strings.Contains(err.Error(), "no matching time found within 10 years") {
		t.Fatalf("unexpected error: %v", err)
	}
}
