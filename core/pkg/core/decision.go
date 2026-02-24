package core

import (
	"fmt"
	"math"
	"time"
)

const defaultMaxAttempts = 64

func Decide(in DecideInput) (Decision, error) {
	identity := in.Identity
	if identity == "" {
		identity = in.Job
	}
	if identity == "" {
		return Decision{}, fmt.Errorf("%w: identity is required", ErrInvalidIdentity)
	}
	if in.Window < 0 {
		return Decision{}, fmt.Errorf("%w: window must be >= 0", ErrInvalidWindow)
	}
	mode := normalizeWindowMode(in.Mode)
	if mode != WindowModeAfter && mode != WindowModeBefore && mode != WindowModeCenter {
		return Decision{}, fmt.Errorf("%w: %q", ErrInvalidWindowMode, in.Mode)
	}
	if in.Dist == "" {
		in.Dist = DistributionUniform
	}
	if in.Dist != DistributionUniform && in.Dist != DistributionSkewEarly && in.Dist != DistributionSkewLate {
		return Decision{}, fmt.Errorf("%w: %q", ErrInvalidDistribution, in.Dist)
	}
	skewShape, err := resolveSkewShape(in.Dist, in.SkewShape)
	if err != nil {
		return Decision{}, err
	}
	if in.SeedStrategy == "" {
		in.SeedStrategy = SeedStrategyStable
	}
	if in.SeedStrategy != SeedStrategyStable && in.SeedStrategy != SeedStrategyDaily && in.SeedStrategy != SeedStrategyWeekly {
		return Decision{}, fmt.Errorf("%w: %q", ErrInvalidSeedStrategy, in.SeedStrategy)
	}
	constraints, err := validateConstraints(in.Constraints)
	if err != nil {
		return Decision{}, err
	}
	maxAttempts := in.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	locationName := in.Timezone
	if locationName == "" {
		locationName = "UTC"
	}
	loc, err := time.LoadLocation(locationName)
	if err != nil {
		return Decision{}, fmt.Errorf("%w: %q", ErrInvalidTimezone, locationName)
	}

	period := in.PeriodStart.UTC()
	periodID := period.Format(time.RFC3339)
	periodKey := derivePeriodKey(period, in.SeedStrategy, loc)
	seedMaterial := fmt.Sprintf("%s\n%s\n%s", identity, periodKey, in.Salt)
	windowStart, windowEnd := computeWindow(period, mode, in.Window)

	startSec := windowStart.Unix()
	endSec := windowEnd.Unix()
	outStart := time.Unix(startSec, 0).UTC()
	outEnd := time.Unix(endSec, 0).UTC()

	hash := SeedHash(seedMaterial)
	rng := NewSplitMix64(SeedUint64(hash))
	chosen := outStart
	attemptCount := 0
	unschedulable := false
	reason := ""
	if endSec > startSec {
		span := endSec - startSec
		found := false
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			u := rng.Float64()
			x := clampUnit(mapDistribution(in.Dist, u, skewShape))
			// +1 permits selecting the exact window end at second granularity.
			offset := int64(math.Floor(x * float64(span+1)))
			if offset > span {
				offset = span
			}
			candidate := time.Unix(startSec+offset, 0).UTC()
			if candidateAllowed(candidate, loc, constraints) {
				chosen = candidate
				attemptCount = attempt
				found = true
				break
			}
		}
		if !found {
			unschedulable = true
			reason = "no candidate accepted within MaxAttempts"
			chosen = time.Time{}
			attemptCount = maxAttempts
		}
	} else {
		if !candidateAllowed(chosen, loc, constraints) {
			unschedulable = true
			reason = "no candidate accepted within MaxAttempts"
			chosen = time.Time{}
		}
	}

	return Decision{
		PeriodID:        periodID,
		NominalTime:     period,
		Job:             in.Job,
		PeriodStart:     period,
		WindowStart:     outStart,
		WindowEnd:       outEnd,
		WindowEndIsOpen: false,
		Mode:            mode,
		Distribution:    in.Dist,
		SeedStrategy:    in.SeedStrategy,
		PeriodKey:       periodKey,
		SeedMaterial:    seedMaterial,
		SeedHash:        SeedHex(seedMaterial),
		ChosenTime:      chosen,
		Unschedulable:   unschedulable,
		Reason:          reason,
		AttemptCount:    attemptCount,
		MaxAttempts:     maxAttempts,
	}, nil
}

func mapDistribution(dist Distribution, u, skewShape float64) float64 {
	switch dist {
	case DistributionSkewEarly:
		return math.Pow(u, skewShape)
	case DistributionSkewLate:
		return 1 - math.Pow(1-u, skewShape)
	case DistributionUniform:
		fallthrough
	default:
		return u
	}
}

func resolveSkewShape(dist Distribution, raw float64) (float64, error) {
	const defaultSkewShape = 2.0

	if dist == DistributionUniform {
		if raw != 0 {
			return 0, fmt.Errorf("%w: skew shape is only supported for skew distributions", ErrInvalidDistribution)
		}
		return defaultSkewShape, nil
	}

	if raw == 0 {
		return defaultSkewShape, nil
	}
	if raw < 0 {
		return 0, fmt.Errorf("%w: skew shape must be > 0", ErrInvalidDistribution)
	}
	return raw, nil
}

func computeWindow(periodStart time.Time, mode WindowMode, window time.Duration) (time.Time, time.Time) {
	switch mode {
	case WindowModeBefore:
		return periodStart.Add(-window), periodStart
	case WindowModeAround:
		fallthrough
	case WindowModeCenter:
		half := window / 2
		start := periodStart.Add(-half)
		return start, start.Add(window)
	case WindowModeAfter:
		fallthrough
	default:
		return periodStart, periodStart.Add(window)
	}
}

func normalizeWindowMode(mode WindowMode) WindowMode {
	if mode == WindowModeAround {
		return WindowModeCenter
	}
	return mode
}

func derivePeriodKey(periodUTC time.Time, strategy SeedStrategy, loc *time.Location) string {
	switch strategy {
	case SeedStrategyDaily:
		return periodUTC.In(loc).Format("2006-01-02")
	case SeedStrategyWeekly:
		y, w := periodUTC.In(loc).ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w)
	case SeedStrategyStable:
		fallthrough
	default:
		return periodUTC.Format(time.RFC3339)
	}
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v >= 1 {
		return math.Nextafter(1, 0)
	}
	return v
}

type constraintRuntime struct {
	onlyHours    map[int]bool
	avoidHours   map[int]bool
	onlyDOW      map[int]bool
	avoidDOW     map[int]bool
	onlyDOM      map[int]bool
	avoidDOM     map[int]bool
	onlyMonths   map[int]bool
	avoidMonths  map[int]bool
	onlyBetween  []TimeRange
	avoidBetween []TimeRange
	onlyDates    []DateRange
	avoidDates   []DateRange
}

func validateConstraints(in ConstraintSpec) (constraintRuntime, error) {
	rt := constraintRuntime{
		onlyHours:   make(map[int]bool, len(in.OnlyHours)),
		avoidHours:  make(map[int]bool, len(in.AvoidHours)),
		onlyDOW:     make(map[int]bool, len(in.OnlyDOW)),
		avoidDOW:    make(map[int]bool, len(in.AvoidDOW)),
		onlyDOM:     make(map[int]bool, len(in.OnlyDOM)),
		avoidDOM:    make(map[int]bool, len(in.AvoidDOM)),
		onlyMonths:  make(map[int]bool, len(in.OnlyMonths)),
		avoidMonths: make(map[int]bool, len(in.AvoidMonths)),
	}

	for _, h := range in.OnlyHours {
		if h < 0 || h > 23 {
			return constraintRuntime{}, fmt.Errorf("%w: invalid only-hours value %d", ErrInvalidConstraint, h)
		}
		rt.onlyHours[h] = true
	}
	for _, h := range in.AvoidHours {
		if h < 0 || h > 23 {
			return constraintRuntime{}, fmt.Errorf("%w: invalid avoid-hours value %d", ErrInvalidConstraint, h)
		}
		rt.avoidHours[h] = true
	}

	for _, d := range in.OnlyDOW {
		if d < 0 || d > 6 {
			return constraintRuntime{}, fmt.Errorf("%w: invalid only-dow value %d", ErrInvalidConstraint, d)
		}
		rt.onlyDOW[d] = true
	}
	for _, d := range in.AvoidDOW {
		if d < 0 || d > 6 {
			return constraintRuntime{}, fmt.Errorf("%w: invalid avoid-dow value %d", ErrInvalidConstraint, d)
		}
		rt.avoidDOW[d] = true
	}

	for _, d := range in.OnlyDOM {
		if d < 1 || d > 31 {
			return constraintRuntime{}, fmt.Errorf("%w: invalid only-dom value %d", ErrInvalidConstraint, d)
		}
		rt.onlyDOM[d] = true
	}
	for _, d := range in.AvoidDOM {
		if d < 1 || d > 31 {
			return constraintRuntime{}, fmt.Errorf("%w: invalid avoid-dom value %d", ErrInvalidConstraint, d)
		}
		rt.avoidDOM[d] = true
	}

	for _, m := range in.OnlyMonths {
		if m < 1 || m > 12 {
			return constraintRuntime{}, fmt.Errorf("%w: invalid only-months value %d", ErrInvalidConstraint, m)
		}
		rt.onlyMonths[m] = true
	}
	for _, m := range in.AvoidMonths {
		if m < 1 || m > 12 {
			return constraintRuntime{}, fmt.Errorf("%w: invalid avoid-months value %d", ErrInvalidConstraint, m)
		}
		rt.avoidMonths[m] = true
	}

	for _, r := range in.OnlyBetween {
		if r.StartMinute < 0 || r.StartMinute > 1439 || r.EndMinute < 0 || r.EndMinute > 1439 {
			return constraintRuntime{}, fmt.Errorf("%w: invalid only-between range %d-%d", ErrInvalidConstraint, r.StartMinute, r.EndMinute)
		}
		if r.StartMinute > r.EndMinute {
			return constraintRuntime{}, fmt.Errorf("%w: invalid only-between range %d-%d", ErrInvalidConstraint, r.StartMinute, r.EndMinute)
		}
		rt.onlyBetween = append(rt.onlyBetween, r)
	}
	for _, r := range in.AvoidBetween {
		if r.StartMinute < 0 || r.StartMinute > 1439 || r.EndMinute < 0 || r.EndMinute > 1439 {
			return constraintRuntime{}, fmt.Errorf("%w: invalid avoid-between range %d-%d", ErrInvalidConstraint, r.StartMinute, r.EndMinute)
		}
		if r.StartMinute > r.EndMinute {
			return constraintRuntime{}, fmt.Errorf("%w: invalid avoid-between range %d-%d", ErrInvalidConstraint, r.StartMinute, r.EndMinute)
		}
		rt.avoidBetween = append(rt.avoidBetween, r)
	}

	for _, r := range in.OnlyDates {
		if r.StartDay <= 0 || r.EndDay <= 0 || r.StartDay > r.EndDay {
			return constraintRuntime{}, fmt.Errorf("%w: invalid only-dates range %d-%d", ErrInvalidConstraint, r.StartDay, r.EndDay)
		}
		rt.onlyDates = append(rt.onlyDates, r)
	}
	for _, r := range in.AvoidDates {
		if r.StartDay <= 0 || r.EndDay <= 0 || r.StartDay > r.EndDay {
			return constraintRuntime{}, fmt.Errorf("%w: invalid avoid-dates range %d-%d", ErrInvalidConstraint, r.StartDay, r.EndDay)
		}
		rt.avoidDates = append(rt.avoidDates, r)
	}

	return rt, nil
}

func candidateAllowed(candidate time.Time, loc *time.Location, constraints constraintRuntime) bool {
	local := candidate.In(loc)
	hour := local.Hour()
	dow := int(local.Weekday())
	dom := local.Day()
	month := int(local.Month())
	minuteOfDay := local.Hour()*60 + local.Minute()
	dayInt := local.Year()*10000 + int(local.Month())*100 + local.Day()

	if len(constraints.onlyHours) > 0 && !constraints.onlyHours[hour] {
		return false
	}
	if len(constraints.onlyDOW) > 0 && !constraints.onlyDOW[dow] {
		return false
	}
	if len(constraints.onlyDOM) > 0 && !constraints.onlyDOM[dom] {
		return false
	}
	if len(constraints.onlyMonths) > 0 && !constraints.onlyMonths[month] {
		return false
	}
	if constraints.avoidHours[hour] {
		return false
	}
	if constraints.avoidDOW[dow] {
		return false
	}
	if constraints.avoidDOM[dom] {
		return false
	}
	if constraints.avoidMonths[month] {
		return false
	}

	if len(constraints.onlyBetween) > 0 {
		ok := false
		for _, r := range constraints.onlyBetween {
			if minuteOfDay >= r.StartMinute && minuteOfDay <= r.EndMinute {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for _, r := range constraints.avoidBetween {
		if minuteOfDay >= r.StartMinute && minuteOfDay <= r.EndMinute {
			return false
		}
	}

	if len(constraints.onlyDates) > 0 {
		ok := false
		for _, r := range constraints.onlyDates {
			if dayInt >= r.StartDay && dayInt <= r.EndDay {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	for _, r := range constraints.avoidDates {
		if dayInt >= r.StartDay && dayInt <= r.EndDay {
			return false
		}
	}
	return true
}
