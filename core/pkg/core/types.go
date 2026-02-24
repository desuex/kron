package core

import "time"

// WindowMode defines how a scheduling window is positioned around period start.
type WindowMode string

const (
	WindowModeAfter  WindowMode = "after"
	WindowModeBefore WindowMode = "before"
	WindowModeAround WindowMode = "around"
	WindowModeCenter WindowMode = "center"
)

// Distribution defines how a time is sampled within the window.
type Distribution string

const (
	DistributionUniform   Distribution = "uniform"
	DistributionSkewEarly Distribution = "skewEarly"
	DistributionSkewLate  Distribution = "skewLate"
)

// SeedStrategy controls how period keying influences deterministic randomness.
type SeedStrategy string

const (
	SeedStrategyStable SeedStrategy = "stable"
	SeedStrategyDaily  SeedStrategy = "daily"
	SeedStrategyWeekly SeedStrategy = "weekly"
)

// TimeRange is an inclusive minute-of-day interval [StartMinute, EndMinute].
type TimeRange struct {
	StartMinute int `json:"startMinute"`
	EndMinute   int `json:"endMinute"`
}

// DateRange is an inclusive day interval represented as YYYYMMDD integers.
type DateRange struct {
	StartDay int `json:"startDay"`
	EndDay   int `json:"endDay"`
}

// ConstraintSpec is the MVP constraint model for deterministic candidate filtering.
type ConstraintSpec struct {
	OnlyHours    []int       `json:"onlyHours,omitempty"`
	AvoidHours   []int       `json:"avoidHours,omitempty"`
	OnlyDOW      []int       `json:"onlyDow,omitempty"`
	AvoidDOW     []int       `json:"avoidDow,omitempty"`
	OnlyDOM      []int       `json:"onlyDom,omitempty"`
	AvoidDOM     []int       `json:"avoidDom,omitempty"`
	OnlyMonths   []int       `json:"onlyMonths,omitempty"`
	AvoidMonths  []int       `json:"avoidMonths,omitempty"`
	OnlyBetween  []TimeRange `json:"onlyBetween,omitempty"`
	AvoidBetween []TimeRange `json:"avoidBetween,omitempty"`
	OnlyDates    []DateRange `json:"onlyDates,omitempty"`
	AvoidDates   []DateRange `json:"avoidDates,omitempty"`
}

// DecideInput is the minimum input set for deterministic decision generation.
type DecideInput struct {
	Identity     string
	Job          string
	PeriodStart  time.Time
	Timezone     string
	Window       time.Duration
	Mode         WindowMode
	Dist         Distribution
	SeedStrategy SeedStrategy
	Salt         string
	MaxAttempts  int
	Constraints  ConstraintSpec
}

// Decision captures the deterministic scheduling output for one period.
type Decision struct {
	PeriodID        string       `json:"periodId"`
	NominalTime     time.Time    `json:"nominalTime"`
	Job             string       `json:"job"`
	PeriodStart     time.Time    `json:"periodStart"`
	WindowStart     time.Time    `json:"windowStart"`
	WindowEnd       time.Time    `json:"windowEnd"`
	WindowEndIsOpen bool         `json:"windowEndIsOpen"`
	Mode            WindowMode   `json:"mode"`
	Distribution    Distribution `json:"distribution"`
	SeedStrategy    SeedStrategy `json:"seedStrategy"`
	PeriodKey       string       `json:"periodKey"`
	SeedMaterial    string       `json:"seedMaterial"`
	SeedHash        string       `json:"seedHash"`
	ChosenTime      time.Time    `json:"chosenTime"`
	Unschedulable   bool         `json:"unschedulable"`
	Reason          string       `json:"reason,omitempty"`
	AttemptCount    int          `json:"attemptCount"`
	MaxAttempts     int          `json:"maxAttempts"`
}
