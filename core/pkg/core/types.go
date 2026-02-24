package core

import "time"

// WindowMode defines how a scheduling window is positioned around period start.
type WindowMode string

const (
	WindowModeAfter  WindowMode = "after"
	WindowModeBefore WindowMode = "before"
	WindowModeCenter WindowMode = "center"
)

// Distribution defines how a time is sampled within the window.
type Distribution string

const (
	DistributionUniform Distribution = "uniform"
)

// DecideInput is the minimum input set for deterministic decision generation.
type DecideInput struct {
	Job         string
	PeriodStart time.Time
	Window      time.Duration
	Mode        WindowMode
	Dist        Distribution
}

// Decision captures the deterministic scheduling output for one period.
type Decision struct {
	Job             string       `json:"job"`
	PeriodStart     time.Time    `json:"periodStart"`
	WindowStart     time.Time    `json:"windowStart"`
	WindowEnd       time.Time    `json:"windowEnd"`
	WindowEndIsOpen bool         `json:"windowEndIsOpen"`
	Mode            WindowMode   `json:"mode"`
	Distribution    Distribution `json:"distribution"`
	SeedMaterial    string       `json:"seedMaterial"`
	SeedHash        string       `json:"seedHash"`
	ChosenTime      time.Time    `json:"chosenTime"`
}
