package daemon

import (
	"context"
	"fmt"
	"testing"
	"time"

	"kron/core/pkg/core"
)

type memStateStore struct {
	states map[string]JobState
}

func newMemStateStore() *memStateStore {
	return &memStateStore{states: map[string]JobState{}}
}

func (m *memStateStore) Load(identity string) (JobState, error) {
	if st, ok := m.states[identity]; ok {
		return st, nil
	}
	return JobState{Version: stateVersion, Identity: identity}, nil
}

func (m *memStateStore) Save(state JobState) error {
	m.states[state.Identity] = state
	return nil
}

type fakeExecutor struct {
	runs int
	err  error
}

func (f *fakeExecutor) Run(context.Context, CommandSpec) (int, error) {
	f.runs++
	if f.err != nil {
		return -1, f.err
	}
	return 0, nil
}

func TestRuntimeExecutesDuePeriodAndPersistsState(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	exec := &fakeExecutor{}

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}

	if err := rt.Step(context.Background(), now); err != nil {
		t.Fatalf("Step error: %v", err)
	}
	if exec.runs != 1 {
		t.Fatalf("run count mismatch: got %d want 1", exec.runs)
	}

	st := store.states[cfg.Identity]
	if st.LastHandledPeriodID != "2026-03-01T00:00:00Z" || st.LastOutcome != OutcomeExecuted {
		t.Fatalf("state mismatch after execute: %+v", st)
	}

	if err := rt.Step(context.Background(), now); err != nil {
		t.Fatalf("second Step error: %v", err)
	}
	if exec.runs != 1 {
		t.Fatalf("expected no duplicate execution, got %d runs", exec.runs)
	}
}

func TestRuntimeMarksMissedWhenPastDeadline(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Policy.Deadline = 10 * time.Second
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	exec := &fakeExecutor{}

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	if err := rt.Step(context.Background(), now); err != nil {
		t.Fatalf("Step error: %v", err)
	}
	if exec.runs != 0 {
		t.Fatalf("expected no execution for missed period, got %d", exec.runs)
	}
	st := store.states[cfg.Identity]
	if st.LastOutcome != OutcomeMissed {
		t.Fatalf("expected missed outcome, got %+v", st)
	}
}

func TestRuntimeMarksUnschedulable(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Constraints.OnlyHours = []int{1}
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	exec := &fakeExecutor{}

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	if err := rt.Step(context.Background(), now); err != nil {
		t.Fatalf("Step error: %v", err)
	}
	if exec.runs != 0 {
		t.Fatalf("expected no execution for unschedulable period, got %d", exec.runs)
	}
	st := store.states[cfg.Identity]
	if st.LastOutcome != OutcomeUnsched {
		t.Fatalf("expected unsched outcome, got %+v", st)
	}
}

func TestRuntimeStartsFromSavedNominalTime(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 1, 30, 0, time.UTC)
	store := newMemStateStore()
	store.states[cfg.Identity] = JobState{
		Version:             stateVersion,
		Identity:            cfg.Identity,
		LastHandledPeriodID: "2026-03-01T00:00:00Z",
		LastNominalTime:     "2026-03-01T00:00:00Z",
		LastOutcome:         OutcomeExecuted,
	}
	exec := &fakeExecutor{}

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	if err := rt.Step(context.Background(), now); err != nil {
		t.Fatalf("Step error: %v", err)
	}
	if exec.runs != 1 {
		t.Fatalf("expected one execution for next nominal period, got %d", exec.runs)
	}
	st := store.states[cfg.Identity]
	if st.LastHandledPeriodID != "2026-03-01T00:01:00Z" {
		t.Fatalf("expected handled period 00:01, got %+v", st)
	}
}

func mustJobConfig(t *testing.T) JobConfig {
	t.Helper()

	spec, err := parseCronSpec([5]string{"*", "*", "*", "*", "*"}, "UTC")
	if err != nil {
		t.Fatalf("parse cron spec: %v", err)
	}
	return JobConfig{
		Identity:  "/etc/krond.d/jobs.kron:backup",
		Name:      "backup",
		Schedule:  spec,
		Command:   CommandSpec{Raw: "/bin/true"},
		Window:    0,
		Mode:      core.WindowModeAfter,
		Dist:      core.DistributionUniform,
		Timezone:  "UTC",
		Seed:      core.SeedStrategyStable,
		Policy:    PolicySpec{Concurrency: DefaultConcurrency},
		SkewShape: 0,
	}
}

func TestRuntimeRecordsSkippedOnExecutorError(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	exec := &fakeExecutor{err: fmt.Errorf("boom")}

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	if err := rt.Step(context.Background(), now); err == nil {
		t.Fatalf("expected execution error")
	}
	st := store.states[cfg.Identity]
	if st.LastOutcome != OutcomeSkipped {
		t.Fatalf("expected skipped outcome after executor error, got %+v", st)
	}
}
