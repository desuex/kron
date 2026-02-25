package daemon

import (
	"time"

	"kron/core/pkg/core"
)

// JobConfig represents one krond job entry loaded from config files.
type JobConfig struct {
	Identity    string
	Name        string
	Schedule    CronSpec
	Command     CommandSpec
	Window      time.Duration
	Mode        core.WindowMode
	Dist        core.Distribution
	SkewShape   float64
	Timezone    string
	Seed        core.SeedStrategy
	Salt        string
	Constraints core.ConstraintSpec
	Policy      PolicySpec
}

// CommandSpec defines how a job process should be started.
type CommandSpec struct {
	Raw     string
	Shell   bool
	Env     []string
	Cwd     string
	User    string
	Group   string
	Timeout time.Duration
}

// PolicySpec captures execution and deadline behavior.
type PolicySpec struct {
	Concurrency string
	Deadline    time.Duration
	Suspend     bool
}

const (
	DefaultConcurrency = "allow"
	OutcomeExecuted    = "executed"
	OutcomeSkipped     = "skipped"
	OutcomeMissed      = "missed"
	OutcomeUnsched     = "unschedulable"
)

// JobState is persisted per identity to keep at-most-once guarantees.
type JobState struct {
	Version             string `json:"version"`
	Identity            string `json:"identity"`
	LastHandledPeriodID string `json:"lastHandledPeriodId"`
	LastOutcome         string `json:"lastOutcome"`
	LastChosenTime      string `json:"lastChosenTime"`
	LastNominalTime     string `json:"lastNominalTime"`
}
