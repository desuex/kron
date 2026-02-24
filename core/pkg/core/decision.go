package core

import (
	"fmt"
	"time"
)

func Decide(in DecideInput) (Decision, error) {
	if in.Job == "" {
		return Decision{}, fmt.Errorf("job is required")
	}
	if in.Window < 0 {
		return Decision{}, fmt.Errorf("window must be >= 0")
	}
	if in.Mode != WindowModeAfter && in.Mode != WindowModeBefore && in.Mode != WindowModeCenter {
		return Decision{}, fmt.Errorf("invalid mode: %q", in.Mode)
	}
	if in.Dist == "" {
		in.Dist = DistributionUniform
	}
	if in.Dist != DistributionUniform {
		return Decision{}, fmt.Errorf("unsupported distribution: %q", in.Dist)
	}

	period := in.PeriodStart.UTC()
	windowStart, windowEnd := computeWindow(period, in.Mode, in.Window)

	startSec := windowStart.Unix()
	endSec := windowEnd.Unix()

	seedMaterial := fmt.Sprintf(
		"job=%s|period=%s|mode=%s|window=%s|dist=%s",
		in.Job,
		period.Format(time.RFC3339),
		in.Mode,
		in.Window,
		in.Dist,
	)

	hash := SeedHash(seedMaterial)
	rng := NewSplitMix64(SeedUint64(hash))
	chosen := time.Unix(startSec, 0).UTC()
	if endSec > startSec {
		span := endSec - startSec
		offset := int64(rng.Uint64() % uint64(span))
		chosen = time.Unix(startSec+offset, 0).UTC()
	}

	return Decision{
		Job:             in.Job,
		PeriodStart:     period,
		WindowStart:     time.Unix(startSec, 0).UTC(),
		WindowEnd:       windowEnd.UTC(),
		WindowEndIsOpen: true,
		Mode:            in.Mode,
		Distribution:    in.Dist,
		SeedMaterial:    seedMaterial,
		SeedHash:        SeedHex(seedMaterial),
		ChosenTime:      chosen,
	}, nil
}

func computeWindow(periodStart time.Time, mode WindowMode, window time.Duration) (time.Time, time.Time) {
	switch mode {
	case WindowModeBefore:
		return periodStart.Add(-window), periodStart
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
