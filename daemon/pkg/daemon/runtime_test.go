package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	goruntime "runtime"
	"strings"
	"sync/atomic"
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

type failingLoadStore struct {
	err error
}

func (f failingLoadStore) Load(string) (JobState, error) {
	return JobState{}, f.err
}

func (f failingLoadStore) Save(JobState) error {
	return nil
}

type failingSaveStore struct {
	states  map[string]JobState
	saveErr error
}

func (f *failingSaveStore) Load(identity string) (JobState, error) {
	if st, ok := f.states[identity]; ok {
		return st, nil
	}
	return JobState{Version: stateVersion, Identity: identity}, nil
}

func (f *failingSaveStore) Save(JobState) error {
	return f.saveErr
}

type nthSaveFailStore struct {
	states    map[string]JobState
	saveErr   error
	failAfter int
	saveCalls int
}

func (n *nthSaveFailStore) Load(identity string) (JobState, error) {
	if st, ok := n.states[identity]; ok {
		return st, nil
	}
	return JobState{Version: stateVersion, Identity: identity}, nil
}

func (n *nthSaveFailStore) Save(state JobState) error {
	n.saveCalls++
	if n.saveCalls > n.failAfter {
		return n.saveErr
	}
	n.states[state.Identity] = state
	return nil
}

type fakeExecutor struct {
	runs int
	err  error
}

func (f *fakeExecutor) Start(context.Context, CommandSpec) (ExecutionHandle, error) {
	f.runs++
	return fakeHandle{err: f.err}, nil
}

type fakeHandle struct {
	err error
}

func (f fakeHandle) PID() int {
	return 1234
}

func (f fakeHandle) Wait() (int, error) {
	if f.err != nil {
		return -1, f.err
	}
	return 0, nil
}

type recordingExecutor struct {
	runs        int
	err         error
	specs       []CommandSpec
	sawDeadline bool
}

func (r *recordingExecutor) Start(ctx context.Context, spec CommandSpec) (ExecutionHandle, error) {
	r.runs++
	r.specs = append(r.specs, spec)
	if _, ok := ctx.Deadline(); ok {
		r.sawDeadline = true
	}
	return fakeHandle{err: r.err}, nil
}

type blockingExecutor struct {
	started chan struct{}
	release chan struct{}
	runs    atomic.Int32
}

func newBlockingExecutor(buffer int) *blockingExecutor {
	return &blockingExecutor{
		started: make(chan struct{}, buffer),
		release: make(chan struct{}, buffer),
	}
}

func (b *blockingExecutor) Start(ctx context.Context, _ CommandSpec) (ExecutionHandle, error) {
	b.runs.Add(1)
	select {
	case b.started <- struct{}{}:
	default:
	}
	return &blockingHandle{
		ctx:     ctx,
		release: b.release,
	}, nil
}

type blockingHandle struct {
	ctx     context.Context
	release chan struct{}
}

func (b *blockingHandle) PID() int {
	return 4321
}

func (b *blockingHandle) Wait() (int, error) {
	select {
	case <-b.release:
		return 0, nil
	case <-b.ctx.Done():
		return 0, nil
	}
}

type deadlineExecutor struct{}

func (deadlineExecutor) Start(ctx context.Context, _ CommandSpec) (ExecutionHandle, error) {
	return deadlineHandle{ctx: ctx}, nil
}

type deadlineHandle struct {
	ctx context.Context
}

func (d deadlineHandle) PID() int {
	return 2468
}

func (d deadlineHandle) Wait() (int, error) {
	<-d.ctx.Done()
	return 0, nil
}

type cancelAwareExecutor struct {
	waitReturned chan struct{}
}

func (c *cancelAwareExecutor) Start(ctx context.Context, _ CommandSpec) (ExecutionHandle, error) {
	return &cancelAwareHandle{
		ctx:          ctx,
		waitReturned: c.waitReturned,
	}, nil
}

type cancelAwareHandle struct {
	ctx          context.Context
	waitReturned chan struct{}
}

func (c *cancelAwareHandle) PID() int {
	return 9876
}

func (c *cancelAwareHandle) Wait() (int, error) {
	<-c.ctx.Done()
	close(c.waitReturned)
	return 0, nil
}

type startFailExecutor struct {
	err error
}

func (s startFailExecutor) Start(context.Context, CommandSpec) (ExecutionHandle, error) {
	return nil, s.err
}

func TestFormatLogEventEscapesValues(t *testing.T) {
	got := formatLogEvent(
		time.Date(2026, 3, 1, 0, 0, 0, 123, time.UTC),
		"INFO",
		"executor",
		"executed",
		logField{key: "job", value: "nightly backup"},
		logField{key: "detail", value: `say "hello"`},
	)
	want := "2026-03-01T00:00:00.000000123Z level=INFO component=executor event=executed job=\"nightly backup\" detail=\"say \\\"hello\\\"\"\n"
	if got != want {
		t.Fatalf("format mismatch:\n got: %q\nwant: %q", got, want)
	}
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
	waitRuntimeIdle(t, rt)
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
	waitRuntimeIdle(t, rt)
	if exec.runs != 1 {
		t.Fatalf("expected no duplicate execution, got %d runs", exec.runs)
	}
}

func TestRuntimeLogsExecutedEvent(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	exec := &fakeExecutor{}
	var logs bytes.Buffer

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now, &logs)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	if err := rt.Step(context.Background(), now); err != nil {
		t.Fatalf("Step error: %v", err)
	}
	waitRuntimeIdle(t, rt)

	text := logs.String()
	if !strings.Contains(text, "event=executed") || !strings.Contains(text, "job=backup") || !strings.Contains(text, "identity="+cfg.Identity) {
		t.Fatalf("expected structured executed event, got %q", text)
	}
}

func TestRuntimePersistsActiveExecutionUntilCompletion(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	exec := newBlockingExecutor(1)

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	if err := rt.Step(context.Background(), now); err != nil {
		t.Fatalf("Step error: %v", err)
	}

	waitStartedRuns(t, exec, 1)
	st := store.states[cfg.Identity]
	if st.ActiveExecution == nil || st.ActiveExecution.PeriodID != "2026-03-01T00:00:00Z" {
		t.Fatalf("expected active execution to be persisted, got %+v", st)
	}

	releaseRuns(exec, 1)
	waitRuntimeIdle(t, rt)

	st = store.states[cfg.Identity]
	if st.ActiveExecution != nil {
		t.Fatalf("expected active execution to clear after completion, got %+v", st.ActiveExecution)
	}
}

func TestRuntimeCancelsRunWhenPersistingActiveExecutionFails(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := &failingSaveStore{
		states:  map[string]JobState{},
		saveErr: errors.New("save active failed"),
	}
	exec := &cancelAwareExecutor{waitReturned: make(chan struct{})}
	var logs bytes.Buffer

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now, &logs)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}

	err = rt.Step(context.Background(), now)
	if err == nil || !strings.Contains(err.Error(), "record active execution") {
		t.Fatalf("expected active execution save error, got %v", err)
	}

	select {
	case <-exec.waitReturned:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for canceled execution to return")
	}
	if !strings.Contains(logs.String(), "event=error") || !strings.Contains(logs.String(), "operation=record_active_execution") {
		t.Fatalf("expected active-execution error log, got %q", logs.String())
	}
}

func TestRuntimeMarksMissedWhenPastDeadline(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Policy.Deadline = 10 * time.Second
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	exec := &fakeExecutor{}
	var logs bytes.Buffer

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now, &logs)
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
	if !strings.Contains(logs.String(), "event=missed") {
		t.Fatalf("expected missed event, got %q", logs.String())
	}
}

func TestRuntimeMarksUnschedulable(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Constraints.OnlyHours = []int{1}
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	exec := &fakeExecutor{}
	var logs bytes.Buffer

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now, &logs)
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
	if !strings.Contains(logs.String(), "event=unschedulable") {
		t.Fatalf("expected unschedulable event, got %q", logs.String())
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
	waitRuntimeIdle(t, rt)
	if exec.runs != 1 {
		t.Fatalf("expected one execution for next nominal period, got %d", exec.runs)
	}
	st := store.states[cfg.Identity]
	if st.LastHandledPeriodID != "2026-03-01T00:01:00Z" {
		t.Fatalf("expected handled period 00:01, got %+v", st)
	}
}

func TestNewRuntimeRecoversStaleActiveExecutionAsSkipped(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 1, 30, 0, time.UTC)
	store := newMemStateStore()
	var logs bytes.Buffer
	store.states[cfg.Identity] = JobState{
		Version:  stateVersion,
		Identity: cfg.Identity,
		ActiveExecution: &ActiveExecutionState{
			PeriodID:    "2026-03-01T00:00:00Z",
			PID:         999999,
			StartedAt:   "2026-03-01T00:00:01Z",
			ChosenTime:  "2026-03-01T00:00:00Z",
			NominalTime: "2026-03-01T00:00:00Z",
		},
	}

	rt, err := newRuntime([]JobConfig{cfg}, store, &fakeExecutor{}, now, &logs)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	if rt.jobs[0].recoveredActive {
		t.Fatalf("expected stale active execution to be cleared")
	}

	st := store.states[cfg.Identity]
	if st.ActiveExecution != nil || st.LastHandledPeriodID != "2026-03-01T00:00:00Z" || st.LastOutcome != OutcomeSkipped {
		t.Fatalf("unexpected recovered state: %+v", st)
	}
	if !strings.Contains(logs.String(), "event=skipped") || !strings.Contains(logs.String(), "reason=recovered_stale_active") {
		t.Fatalf("expected recovered skip event, got %q", logs.String())
	}
}

func TestRuntimeRecoveredActiveForbidSkipsLaterPeriods(t *testing.T) {
	if goruntime.GOOS == "windows" {
		t.Skip("recovered pid liveness checks are unix-first in this pass")
	}

	cfg := mustJobConfig(t)
	cfg.Policy.Concurrency = "forbid"
	now := time.Date(2026, 3, 1, 0, 1, 30, 0, time.UTC)
	store := newMemStateStore()
	store.states[cfg.Identity] = JobState{
		Version:  stateVersion,
		Identity: cfg.Identity,
		ActiveExecution: &ActiveExecutionState{
			PeriodID:    "2026-03-01T00:00:00Z",
			PID:         os.Getpid(),
			StartedAt:   "2026-03-01T00:00:01Z",
			ChosenTime:  "2026-03-01T00:00:00Z",
			NominalTime: "2026-03-01T00:00:00Z",
		},
	}

	rt, err := newRuntime([]JobConfig{cfg}, store, &fakeExecutor{}, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	if !rt.jobs[0].recoveredActive {
		t.Fatalf("expected live recovered execution to be tracked")
	}

	if err := rt.Step(context.Background(), now); err != nil {
		t.Fatalf("Step error: %v", err)
	}

	st := store.states[cfg.Identity]
	if st.ActiveExecution == nil || st.LastHandledPeriodID != "2026-03-01T00:01:00Z" || st.LastOutcome != OutcomeSkipped {
		t.Fatalf("expected later period skip while recovered execution is live, got %+v", st)
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
	if err := rt.Step(context.Background(), now); err != nil {
		t.Fatalf("unexpected Step error before completion: %v", err)
	}
	if err := rt.waitInFlight(context.Background()); err == nil {
		t.Fatalf("expected execution error")
	}
	st := store.states[cfg.Identity]
	if st.LastOutcome != OutcomeSkipped {
		t.Fatalf("expected skipped outcome after executor error, got %+v", st)
	}
}

func TestRuntimePassesCommandIdentityToExecutor(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Command = CommandSpec{
		Raw:     "/bin/echo hello",
		Shell:   false,
		User:    "svc",
		Group:   "ops",
		Cwd:     "/tmp",
		Env:     []string{"FOO=bar"},
		Timeout: 2 * time.Second,
	}

	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	exec := &recordingExecutor{}

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	if err := rt.Step(context.Background(), now); err != nil {
		t.Fatalf("Step error: %v", err)
	}
	waitRuntimeIdle(t, rt)
	if exec.runs != 1 {
		t.Fatalf("run count mismatch: got %d want 1", exec.runs)
	}
	if len(exec.specs) != 1 {
		t.Fatalf("expected one recorded command spec, got %d", len(exec.specs))
	}

	got := exec.specs[0]
	if got.Raw != cfg.Command.Raw || got.User != cfg.Command.User || got.Group != cfg.Command.Group {
		t.Fatalf("identity fields mismatch: got=%+v want=%+v", got, cfg.Command)
	}
	if got.Cwd != cfg.Command.Cwd || got.Timeout != cfg.Command.Timeout || got.Shell != cfg.Command.Shell {
		t.Fatalf("command options mismatch: got=%+v want=%+v", got, cfg.Command)
	}
	if len(got.Env) != 1 || got.Env[0] != "FOO=bar" {
		t.Fatalf("env mismatch: got=%v", got.Env)
	}
	if !exec.sawDeadline {
		t.Fatalf("expected runtime to pass timeout-backed context deadline to executor")
	}
}

func TestRuntimeWithoutCommandTimeoutHasNoDeadline(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Command.Timeout = 0

	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	exec := &recordingExecutor{}

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	if err := rt.Step(context.Background(), now); err != nil {
		t.Fatalf("Step error: %v", err)
	}
	waitRuntimeIdle(t, rt)
	if exec.runs != 1 {
		t.Fatalf("run count mismatch: got %d want 1", exec.runs)
	}
	if exec.sawDeadline {
		t.Fatalf("did not expect deadline when command timeout is unset")
	}
}

func TestNewRuntimeLoadError(t *testing.T) {
	cfg := mustJobConfig(t)
	sentinel := errors.New("load failed")
	_, err := newRuntime(
		[]JobConfig{cfg},
		failingLoadStore{err: sentinel},
		&fakeExecutor{},
		time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC),
	)
	if err == nil {
		t.Fatalf("expected load error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped load error, got %v", err)
	}
}

func TestNextPeriodFromStateFallsBackOnInvalidSavedTime(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 12, 34, 56, 0, time.UTC)
	st := JobState{LastNominalTime: "definitely-not-rfc3339"}

	got, err := nextPeriodFromState(cfg.Schedule, st, now)
	if err != nil {
		t.Fatalf("nextPeriodFromState error: %v", err)
	}
	want, err := cfg.Schedule.NextAfter(now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("schedule next error: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("next period mismatch: got=%s want=%s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestRuntimeStepNoopWhenChosenTimeInFuture(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	store := newMemStateStore()
	exec := &fakeExecutor{}

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}

	job := rt.jobs[0]
	job.decisionReady = true
	job.decision = core.Decision{
		PeriodID:    "future-period",
		PeriodStart: now,
		ChosenTime:  now.Add(10 * time.Minute),
	}

	if err := rt.stepJob(context.Background(), job, now); err != nil {
		t.Fatalf("stepJob error: %v", err)
	}
	if exec.runs != 0 {
		t.Fatalf("expected no execution when chosen time is in future, got %d", exec.runs)
	}
	if st := store.states[cfg.Identity]; st.LastHandledPeriodID != "" {
		t.Fatalf("expected no state update, got %+v", st)
	}
}

func TestRuntimeStepRespectsCatchupLimitPerStep(t *testing.T) {
	cfg := mustJobConfig(t)
	initNow := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	stepNow := initNow.Add(40 * time.Minute)
	store := newMemStateStore()
	exec := &fakeExecutor{}

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, initNow)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}

	if err := rt.Step(context.Background(), stepNow); err != nil {
		t.Fatalf("first Step error: %v", err)
	}
	waitRuntimeIdle(t, rt)
	if exec.runs != maxJobCatchupPerStep {
		t.Fatalf("first Step run count mismatch: got %d want %d", exec.runs, maxJobCatchupPerStep)
	}

	if err := rt.Step(context.Background(), stepNow); err != nil {
		t.Fatalf("second Step error: %v", err)
	}
	waitRuntimeIdle(t, rt)
	if exec.runs != 41 {
		t.Fatalf("expected remaining backlog on second Step, got total runs %d", exec.runs)
	}
}

func TestRuntimeStepReturnsDecisionError(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Mode = core.WindowMode("bad-mode")
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	exec := &fakeExecutor{}

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	err = rt.Step(context.Background(), now)
	if err == nil {
		t.Fatalf("expected decision error")
	}
	if !strings.Contains(err.Error(), "invalid window mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRuntimeLogsErrorOnExecutionStartFailure(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	var logs bytes.Buffer

	rt, err := newRuntime([]JobConfig{cfg}, store, startFailExecutor{err: errors.New("boom")}, now, &logs)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	err = rt.Step(context.Background(), now)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected start failure, got %v", err)
	}
	if !strings.Contains(logs.String(), "event=error") || !strings.Contains(logs.String(), "operation=start_execution") {
		t.Fatalf("expected start error event, got %q", logs.String())
	}
}

func TestRuntimeReturnsRecordExecutedOutcomeErrorOnSaveFailure(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	sentinel := errors.New("save failed")
	store := &nthSaveFailStore{
		states:    map[string]JobState{},
		saveErr:   sentinel,
		failAfter: 1,
	}
	exec := &fakeExecutor{}

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	err = rt.Step(context.Background(), now)
	if err != nil {
		t.Fatalf("unexpected Step error before completion: %v", err)
	}
	err = rt.waitInFlight(context.Background())
	if err == nil {
		t.Fatalf("expected save error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped save error, got %v", err)
	}
	if !strings.Contains(err.Error(), "record executed outcome") {
		t.Fatalf("expected executed outcome context, got %v", err)
	}
}

func TestRuntimeReturnsRecordSkippedOutcomeErrorOnSaveFailure(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	sentinel := errors.New("save failed")
	store := &nthSaveFailStore{
		states:    map[string]JobState{},
		saveErr:   sentinel,
		failAfter: 1,
	}
	exec := &fakeExecutor{err: errors.New("run failed")}

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	err = rt.Step(context.Background(), now)
	if err != nil {
		t.Fatalf("unexpected Step error before completion: %v", err)
	}
	err = rt.waitInFlight(context.Background())
	if err == nil {
		t.Fatalf("expected save error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped save error, got %v", err)
	}
	if !strings.Contains(err.Error(), "record skipped outcome") {
		t.Fatalf("expected skipped outcome context, got %v", err)
	}
}

func TestRuntimeSkipsSuspendedJob(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Policy.Suspend = true
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
		t.Fatalf("expected no execution for suspended job, got %d", exec.runs)
	}
	if st := store.states[cfg.Identity]; st.LastHandledPeriodID != "" {
		t.Fatalf("expected no state update for suspended job, got %+v", st)
	}
}

func TestRuntimeStepErrorIncludesIdentity(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Mode = core.WindowMode("invalid")
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()

	rt, err := newRuntime([]JobConfig{cfg}, store, &fakeExecutor{}, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	err = rt.Step(context.Background(), now)
	if err == nil {
		t.Fatalf("expected step error")
	}
	if !strings.Contains(err.Error(), "job "+cfg.Identity+":") {
		t.Fatalf("expected identity in wrapped error, got %v", err)
	}
}

func TestNewRuntimeReturnsErrorWhenScheduleHasNoFutureMatches(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Schedule = impossibleCronSpec()

	_, err := newRuntime(
		[]JobConfig{cfg},
		newMemStateStore(),
		&fakeExecutor{},
		time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC),
	)
	if err == nil {
		t.Fatalf("expected schedule computation error")
	}
	if !strings.Contains(err.Error(), "compute next period") {
		t.Fatalf("expected compute next period context, got %v", err)
	}
}

func TestStepJobAdvancesWhenDecisionAlreadyHandled(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	exec := &fakeExecutor{}

	rt, err := newRuntime([]JobConfig{cfg}, newMemStateStore(), exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}

	job := rt.jobs[0]
	oldNext := now.Add(24 * time.Hour)
	job.nextPeriod = oldNext
	job.state.LastHandledPeriodID = "period-a"
	job.decisionReady = true
	job.decision = core.Decision{
		PeriodID:    "period-a",
		PeriodStart: oldNext,
		ChosenTime:  oldNext.Add(10 * time.Minute),
	}

	if err := rt.stepJob(context.Background(), job, now); err != nil {
		t.Fatalf("stepJob error: %v", err)
	}
	if exec.runs != 0 {
		t.Fatalf("expected no execution, got %d", exec.runs)
	}
	if !job.nextPeriod.After(oldNext) {
		t.Fatalf("expected nextPeriod to advance, old=%s new=%s", oldNext, job.nextPeriod)
	}
}

func TestStepJobReturnsAdvancePeriodError(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)

	rt, err := newRuntime([]JobConfig{cfg}, newMemStateStore(), &fakeExecutor{}, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}

	job := rt.jobs[0]
	job.cfg.Schedule = impossibleCronSpec()
	job.state.LastHandledPeriodID = "period-b"
	job.decisionReady = true
	job.decision = core.Decision{PeriodID: "period-b"}

	err = rt.stepJob(context.Background(), job, now)
	if err == nil {
		t.Fatalf("expected advance period error")
	}
	if !strings.Contains(err.Error(), "no matching time found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStepJobUnschedRecordOutcomeError(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	sentinel := errors.New("save failed")
	store := &failingSaveStore{states: map[string]JobState{}, saveErr: sentinel}

	rt, err := newRuntime([]JobConfig{cfg}, store, &fakeExecutor{}, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}

	job := rt.jobs[0]
	job.decisionReady = true
	job.decision = core.Decision{PeriodID: "period-unsched", PeriodStart: now, Unschedulable: true}

	err = rt.stepJob(context.Background(), job, now)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected save error, got %v", err)
	}
}

func TestStepJobUnschedAdvanceError(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)

	rt, err := newRuntime([]JobConfig{cfg}, newMemStateStore(), &fakeExecutor{}, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}

	job := rt.jobs[0]
	job.cfg.Schedule = impossibleCronSpec()
	job.decisionReady = true
	job.decision = core.Decision{PeriodID: "period-unsched", PeriodStart: now, Unschedulable: true}

	err = rt.stepJob(context.Background(), job, now)
	if err == nil || !strings.Contains(err.Error(), "no matching time found") {
		t.Fatalf("expected advance error, got %v", err)
	}
}

func TestStepJobMissedRecordOutcomeError(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Policy.Deadline = time.Second
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	sentinel := errors.New("save failed")
	store := &failingSaveStore{states: map[string]JobState{}, saveErr: sentinel}

	rt, err := newRuntime([]JobConfig{cfg}, store, &fakeExecutor{}, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}

	job := rt.jobs[0]
	job.decisionReady = true
	job.decision = core.Decision{PeriodID: "period-missed", PeriodStart: now.Add(-2 * time.Minute), ChosenTime: now.Add(-2 * time.Minute)}

	err = rt.stepJob(context.Background(), job, now)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected save error, got %v", err)
	}
}

func TestStepJobMissedAdvanceError(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Policy.Deadline = time.Second
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)

	rt, err := newRuntime([]JobConfig{cfg}, newMemStateStore(), &fakeExecutor{}, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}

	job := rt.jobs[0]
	job.cfg.Schedule = impossibleCronSpec()
	job.decisionReady = true
	job.decision = core.Decision{PeriodID: "period-missed", PeriodStart: now.Add(-2 * time.Minute), ChosenTime: now.Add(-2 * time.Minute)}

	err = rt.stepJob(context.Background(), job, now)
	if err == nil || !strings.Contains(err.Error(), "no matching time found") {
		t.Fatalf("expected advance error, got %v", err)
	}
}

func TestStepJobExecuteAdvanceError(t *testing.T) {
	cfg := mustJobConfig(t)
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	exec := &fakeExecutor{}

	rt, err := newRuntime([]JobConfig{cfg}, newMemStateStore(), exec, now)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}

	job := rt.jobs[0]
	job.cfg.Schedule = impossibleCronSpec()
	job.decisionReady = true
	job.decision = core.Decision{PeriodID: "period-exec", PeriodStart: now.Add(-time.Minute), ChosenTime: now.Add(-time.Minute)}

	err = rt.stepJob(context.Background(), job, now)
	if err == nil || !strings.Contains(err.Error(), "no matching time found") {
		t.Fatalf("expected advance error, got %v", err)
	}
	if exec.runs != 1 {
		t.Fatalf("expected execute to run once before advance error, got %d", exec.runs)
	}
}

func TestExecuteHandlesDeadlineExceededAsExecuted(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Command.Timeout = 5 * time.Millisecond
	now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	store := newMemStateStore()
	var logs bytes.Buffer
	rt := &runtime{store: store, executor: deadlineExecutor{}, logWriter: &logs}

	job := &runtimeJob{cfg: cfg, state: JobState{Version: stateVersion, Identity: cfg.Identity}}
	decision := core.Decision{PeriodID: "period-timeout", PeriodStart: now, ChosenTime: now}

	if err := rt.execute(context.Background(), job, decision); err != nil {
		t.Fatalf("execute error: %v", err)
	}
	st := store.states[cfg.Identity]
	if st.LastOutcome != OutcomeExecuted {
		t.Fatalf("expected executed outcome for timeout-handled run, got %+v", st)
	}
	if !strings.Contains(logs.String(), "event=executed") || !strings.Contains(logs.String(), "timed_out=true") {
		t.Fatalf("expected timed out executed event, got %q", logs.String())
	}
}

func TestRuntimeConcurrencyAllowOverlaps(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Policy.Concurrency = "allow"

	initNow := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	stepNow := initNow.Add(time.Minute)
	store := newMemStateStore()
	exec := newBlockingExecutor(4)

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, initNow)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	if err := rt.Step(context.Background(), stepNow); err != nil {
		t.Fatalf("Step error: %v", err)
	}

	waitStartedRuns(t, exec, 2)
	if rt.running != 2 {
		t.Fatalf("expected two in-flight runs for allow mode, got %d", rt.running)
	}
	releaseRuns(exec, 2)
	if err := rt.waitInFlight(context.Background()); err != nil {
		t.Fatalf("waitInFlight error: %v", err)
	}

	st := store.states[cfg.Identity]
	if st.LastHandledPeriodID != "2026-03-01T00:01:00Z" || st.LastOutcome != OutcomeExecuted {
		t.Fatalf("unexpected final state: %+v", st)
	}
	if st.ActiveExecution != nil {
		t.Fatalf("expected no active execution after completions, got %+v", st.ActiveExecution)
	}
	if got := int(exec.runs.Load()); got != 2 {
		t.Fatalf("expected two executions, got %d", got)
	}
}

func TestRuntimeConcurrencyForbidSkipsWhenRunning(t *testing.T) {
	cfg := mustJobConfig(t)
	cfg.Policy.Concurrency = "forbid"

	initNow := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
	stepNow := initNow.Add(time.Minute)
	store := newMemStateStore()
	exec := newBlockingExecutor(2)
	var logs bytes.Buffer

	rt, err := newRuntime([]JobConfig{cfg}, store, exec, initNow, &logs)
	if err != nil {
		t.Fatalf("newRuntime error: %v", err)
	}
	if err := rt.Step(context.Background(), stepNow); err != nil {
		t.Fatalf("Step error: %v", err)
	}

	waitStartedRuns(t, exec, 1)
	if rt.running != 1 {
		t.Fatalf("expected one in-flight run for forbid mode, got %d", rt.running)
	}
	st := store.states[cfg.Identity]
	if st.LastHandledPeriodID != "2026-03-01T00:01:00Z" || st.LastOutcome != OutcomeSkipped {
		t.Fatalf("expected second period skipped while first is in-flight, got %+v", st)
	}

	releaseRuns(exec, 1)
	if err := rt.waitInFlight(context.Background()); err != nil {
		t.Fatalf("waitInFlight error: %v", err)
	}

	st = store.states[cfg.Identity]
	if st.LastHandledPeriodID != "2026-03-01T00:01:00Z" || st.LastOutcome != OutcomeSkipped {
		t.Fatalf("expected stale completion to not rewind state, got %+v", st)
	}
	if st.ActiveExecution != nil {
		t.Fatalf("expected stale completion to clear active execution, got %+v", st.ActiveExecution)
	}
	if got := int(exec.runs.Load()); got != 1 {
		t.Fatalf("expected one execution in forbid mode, got %d", got)
	}
	if !strings.Contains(logs.String(), "event=skipped") || !strings.Contains(logs.String(), "reason=concurrency_forbid") {
		t.Fatalf("expected concurrency skip event, got %q", logs.String())
	}
}

func waitStartedRuns(t *testing.T, exec *blockingExecutor, want int) {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for i := 0; i < want; i++ {
		select {
		case <-exec.started:
		case <-timeout:
			t.Fatalf("timed out waiting for started run %d/%d", i+1, want)
		}
	}
}

func releaseRuns(exec *blockingExecutor, n int) {
	for i := 0; i < n; i++ {
		exec.release <- struct{}{}
	}
}

func waitRuntimeIdle(t *testing.T, rt *runtime) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rt.waitInFlight(ctx); err != nil {
		t.Fatalf("waitInFlight error: %v", err)
	}
}

func TestWrapRecordOutcomeErrorVariants(t *testing.T) {
	base := errors.New("state save failed")
	if err := wrapRecordOutcomeError(OutcomeSkipped, base); !strings.Contains(err.Error(), "record skipped outcome") {
		t.Fatalf("unexpected skipped wrap: %v", err)
	}
	if err := wrapRecordOutcomeError(OutcomeMissed, base); !strings.Contains(err.Error(), "record missed outcome") {
		t.Fatalf("unexpected missed wrap: %v", err)
	}
	if err := wrapRecordOutcomeError(OutcomeUnsched, base); !strings.Contains(err.Error(), "record unsched outcome") {
		t.Fatalf("unexpected unsched wrap: %v", err)
	}
	if err := wrapRecordOutcomeError("other", base); !strings.Contains(err.Error(), "record executed outcome") {
		t.Fatalf("unexpected default wrap: %v", err)
	}
}

func TestWaitInFlightContextCanceled(t *testing.T) {
	rt := &runtime{
		completions: make(chan runCompletion),
		running:     1,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := rt.waitInFlight(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func impossibleCronSpec() CronSpec {
	var spec CronSpec
	spec.location = time.UTC
	spec.minutes[0] = true
	spec.hours[0] = true
	spec.dom[1] = true
	spec.dow[0] = true
	return spec
}
