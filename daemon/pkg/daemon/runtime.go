package daemon

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"kron/core/pkg/core"
)

const maxJobCatchupPerStep = 32

type StartOptions struct {
	ConfigPath string
	LogWriter  io.Writer
	StateDir   string
	Tick       time.Duration
	Once       bool
	Source     string
}

type runtime struct {
	jobs        []*runtimeJob
	store       StateStore
	executor    Executor
	completions chan runCompletion
	logWriter   io.Writer
	running     int
}

type runtimeJob struct {
	cfg             JobConfig
	state           JobState
	nextPeriod      time.Time
	decision        core.Decision
	decisionReady   bool
	running         int
	recoveredActive bool
}

type runCompletion struct {
	job      *runtimeJob
	decision core.Decision
	outcome  string
	err      error
	pid      int
	timedOut bool
}

func Start(ctx context.Context, opts StartOptions) (retErr error) {
	if opts.ConfigPath == "" {
		return fmt.Errorf("config path is required")
	}
	if opts.Source == "" {
		opts.Source = "kron"
	}
	if opts.StateDir == "" {
		opts.StateDir = ".krond-state"
	}
	if opts.Tick <= 0 {
		opts.Tick = time.Second
	}
	if opts.LogWriter == nil {
		opts.LogWriter = os.Stderr
	}

	lock, err := acquireStateLock(opts.StateDir)
	if err != nil {
		return err
	}
	defer func() {
		if err := lock.Release(); err != nil && retErr == nil {
			retErr = fmt.Errorf("release state lock: %w", err)
		}
	}()

	jobs, err := loadJobsBySource(opts.Source, opts.ConfigPath)
	if err != nil {
		return err
	}

	rt, err := newRuntime(jobs, FileStateStore{Dir: opts.StateDir}, OSExecutor{}, time.Now().UTC(), opts.LogWriter)
	if err != nil {
		return err
	}

	if opts.Once {
		if err := rt.Step(ctx, time.Now().UTC()); err != nil {
			return err
		}
		return rt.waitInFlight(ctx)
	}

	ticker := time.NewTicker(opts.Tick)
	defer ticker.Stop()

	for {
		if err := rt.Step(ctx, time.Now().UTC()); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func loadJobsBySource(source, configPath string) ([]JobConfig, error) {
	switch source {
	case "kron":
		return LoadJobs(configPath)
	case "cron":
		return LoadSystemCron(configPath)
	default:
		return nil, fmt.Errorf("unsupported source %q", source)
	}
}

func newRuntime(jobs []JobConfig, store StateStore, executor Executor, now time.Time, logWriter ...io.Writer) (*runtime, error) {
	completionCap := len(jobs) * maxJobCatchupPerStep
	if completionCap < 64 {
		completionCap = 64
	}
	dest := io.Writer(io.Discard)
	if len(logWriter) > 0 && logWriter[0] != nil {
		dest = logWriter[0]
	}

	rt := &runtime{
		jobs:        make([]*runtimeJob, 0, len(jobs)),
		store:       store,
		executor:    executor,
		completions: make(chan runCompletion, completionCap),
		logWriter:   dest,
	}

	for _, cfg := range jobs {
		st, err := store.Load(cfg.Identity)
		if err != nil {
			return nil, fmt.Errorf("load state for %s: %w", cfg.Identity, err)
		}
		recoveredActive, st, err := reconcileStartupState(store, cfg, st, dest)
		if err != nil {
			return nil, fmt.Errorf("reconcile state for %s: %w", cfg.Identity, err)
		}
		next, err := nextPeriodFromState(cfg.Schedule, st, now)
		if err != nil {
			return nil, fmt.Errorf("compute next period for %s: %w", cfg.Identity, err)
		}

		rt.jobs = append(rt.jobs, &runtimeJob{
			cfg:             cfg,
			state:           st,
			nextPeriod:      next,
			recoveredActive: recoveredActive,
		})
	}
	return rt, nil
}

func nextPeriodFromState(schedule CronSpec, st JobState, now time.Time) (time.Time, error) {
	if st.ActiveExecution != nil && st.ActiveExecution.NominalTime != "" {
		activeNominal, err := time.Parse(time.RFC3339, st.ActiveExecution.NominalTime)
		if err == nil {
			return schedule.NextAfter(activeNominal)
		}
	}
	if st.LastNominalTime != "" {
		lastNominal, err := time.Parse(time.RFC3339, st.LastNominalTime)
		if err == nil {
			return schedule.NextAfter(lastNominal)
		}
	}
	return schedule.NextAfter(now.Add(-time.Minute))
}

func (r *runtime) Step(ctx context.Context, now time.Time) error {
	if err := r.drainCompletionsNonBlocking(); err != nil {
		return err
	}

	for _, job := range r.jobs {
		if job.cfg.Policy.Suspend {
			continue
		}
		if err := r.reconcileRecoveredActive(job); err != nil {
			return fmt.Errorf("job %s: %w", job.cfg.Identity, err)
		}
		if err := r.stepJob(ctx, job, now); err != nil {
			return fmt.Errorf("job %s: %w", job.cfg.Identity, err)
		}
	}

	return r.drainCompletionsNonBlocking()
}

func (r *runtime) stepJob(ctx context.Context, job *runtimeJob, now time.Time) error {
	for i := 0; i < maxJobCatchupPerStep; i++ {
		if !job.decisionReady {
			decision, err := core.Decide(core.DecideInput{
				Identity:     job.cfg.Identity,
				Job:          job.cfg.Name,
				PeriodStart:  job.nextPeriod,
				Timezone:     job.cfg.Timezone,
				Window:       job.cfg.Window,
				Mode:         job.cfg.Mode,
				Dist:         job.cfg.Dist,
				SkewShape:    job.cfg.SkewShape,
				SeedStrategy: job.cfg.Seed,
				Salt:         job.cfg.Salt,
				Constraints:  job.cfg.Constraints,
			})
			if err != nil {
				return err
			}
			job.decision = decision
			job.decisionReady = true
		}

		decision := job.decision
		if job.state.ActiveExecution != nil && job.state.ActiveExecution.PeriodID == decision.PeriodID {
			return nil
		}
		if job.state.LastHandledPeriodID == decision.PeriodID {
			if err := r.advancePeriod(job); err != nil {
				return err
			}
			continue
		}

		if decision.Unschedulable {
			if err := r.recordOutcome(job, decision, OutcomeUnsched); err != nil {
				r.logError(job, decision, "record_outcome", err)
				return err
			}
			r.logUnschedulable(job, decision)
			if err := r.advancePeriod(job); err != nil {
				return err
			}
			continue
		}

		if decision.ChosenTime.After(now) {
			return nil
		}

		if job.cfg.Policy.Deadline > 0 && now.After(decision.ChosenTime.Add(job.cfg.Policy.Deadline)) {
			if err := r.recordOutcome(job, decision, OutcomeMissed); err != nil {
				r.logError(job, decision, "record_outcome", err)
				return err
			}
			r.logMissed(job, decision, now)
			if err := r.advancePeriod(job); err != nil {
				return err
			}
			continue
		}

		if job.cfg.Policy.Concurrency == "forbid" && job.activeRuns() > 0 {
			if err := r.recordOutcome(job, decision, OutcomeSkipped); err != nil {
				r.logError(job, decision, "record_outcome", err)
				return err
			}
			r.logSkipped(job, decision, "concurrency_forbid", activePID(job))
			if err := r.advancePeriod(job); err != nil {
				return err
			}
			continue
		}

		if err := r.dispatchRun(ctx, job, decision, now); err != nil {
			return err
		}
		if err := r.advancePeriod(job); err != nil {
			return err
		}
	}
	return nil
}

func (r *runtime) recordOutcome(job *runtimeJob, decision core.Decision, outcome string) error {
	job.state.Identity = job.cfg.Identity
	job.state.Version = stateVersion
	job.state.LastHandledPeriodID = decision.PeriodID
	job.state.LastOutcome = outcome
	job.state.LastNominalTime = decision.PeriodStart.UTC().Format(time.RFC3339)
	if decision.ChosenTime.IsZero() {
		job.state.LastChosenTime = ""
	} else {
		job.state.LastChosenTime = decision.ChosenTime.UTC().Format(time.RFC3339)
	}
	if job.state.ActiveExecution != nil && job.state.ActiveExecution.PeriodID == decision.PeriodID {
		job.state.ActiveExecution = nil
		job.recoveredActive = false
	}
	return r.store.Save(job.state)
}

func wrapRecordOutcomeError(outcome string, err error) error {
	switch outcome {
	case OutcomeSkipped:
		return fmt.Errorf("record skipped outcome: %w", err)
	case OutcomeMissed:
		return fmt.Errorf("record missed outcome: %w", err)
	case OutcomeUnsched:
		return fmt.Errorf("record unsched outcome: %w", err)
	default:
		return fmt.Errorf("record executed outcome: %w", err)
	}
}

func (r *runtime) dispatchRun(ctx context.Context, job *runtimeJob, decision core.Decision, now time.Time) error {
	runCtx, cancel := commandContext(ctx, job.cfg.Command.Timeout)
	handle, err := r.executor.Start(runCtx, job.cfg.Command)
	if err != nil {
		cancel()
		r.logError(job, decision, "start_execution", err)
		return err
	}

	job.state.ActiveExecution = &ActiveExecutionState{
		PeriodID:    decision.PeriodID,
		PID:         handle.PID(),
		StartedAt:   now.UTC().Format(time.RFC3339),
		ChosenTime:  decision.ChosenTime.UTC().Format(time.RFC3339),
		NominalTime: decision.PeriodStart.UTC().Format(time.RFC3339),
	}
	job.recoveredActive = false
	if err := r.store.Save(job.state); err != nil {
		job.state.ActiveExecution = nil
		cancel()
		_, _ = handle.Wait()
		r.logError(job, decision, "record_active_execution", err)
		return fmt.Errorf("record active execution: %w", err)
	}

	job.running++
	r.running++

	go func(job *runtimeJob, decision core.Decision, runCtx context.Context, cancel context.CancelFunc, handle ExecutionHandle) {
		defer cancel()
		_, err := handle.Wait()
		outcome, runErr, timedOut := classifyExecutionResult(runCtx, err)
		r.completions <- runCompletion{
			job:      job,
			decision: decision,
			outcome:  outcome,
			err:      runErr,
			pid:      handle.PID(),
			timedOut: timedOut,
		}
	}(job, decision, runCtx, cancel, handle)

	return nil
}

func (r *runtime) handleCompletion(completion runCompletion) error {
	if completion.job.running > 0 {
		completion.job.running--
	}
	if r.running > 0 {
		r.running--
	}

	// Ignore stale completions that arrive after a newer handled period.
	if completion.job.state.LastHandledPeriodID != "" && completion.decision.PeriodID < completion.job.state.LastHandledPeriodID {
		if completion.job.state.ActiveExecution != nil && completion.job.state.ActiveExecution.PeriodID == completion.decision.PeriodID {
			completion.job.state = clearRecoveredExecution(completion.job.state)
			completion.job.recoveredActive = false
			if err := r.store.Save(completion.job.state); err != nil {
				r.logError(completion.job, completion.decision, "record_outcome", err)
				return fmt.Errorf("job %s: %w", completion.job.cfg.Identity, wrapRecordOutcomeError(completion.outcome, err))
			}
		}
		return nil
	}

	if err := r.recordOutcome(completion.job, completion.decision, completion.outcome); err != nil {
		r.logError(completion.job, completion.decision, "record_outcome", err)
		return fmt.Errorf("job %s: %w", completion.job.cfg.Identity, wrapRecordOutcomeError(completion.outcome, err))
	}
	r.logCompletionOutcome(completion)
	if completion.err != nil {
		r.logError(completion.job, completion.decision, "execute_command", completion.err)
		return fmt.Errorf("job %s: %w", completion.job.cfg.Identity, completion.err)
	}
	return nil
}

func (r *runtime) drainCompletionsNonBlocking() error {
	for {
		select {
		case completion := <-r.completions:
			if err := r.handleCompletion(completion); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func (r *runtime) waitInFlight(ctx context.Context) error {
	for r.running > 0 {
		select {
		case completion := <-r.completions:
			if err := r.handleCompletion(completion); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return r.drainCompletionsNonBlocking()
}

func (r *runtime) advancePeriod(job *runtimeJob) error {
	next, err := job.cfg.Schedule.NextAfter(job.nextPeriod)
	if err != nil {
		return err
	}
	job.nextPeriod = next
	job.decisionReady = false
	return nil
}

func (job *runtimeJob) activeRuns() int {
	runs := job.running
	if job.recoveredActive {
		runs++
	}
	return runs
}

func commandContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}

func (r *runtime) execute(ctx context.Context, job *runtimeJob, decision core.Decision) error {
	runCtx, cancel := commandContext(ctx, job.cfg.Command.Timeout)
	defer cancel()

	handle, err := r.executor.Start(runCtx, job.cfg.Command)
	if err != nil {
		r.logError(job, decision, "start_execution", err)
		return err
	}
	_, err = handle.Wait()
	outcome, runErr, timedOut := classifyExecutionResult(runCtx, err)
	if saveErr := r.recordOutcome(job, decision, outcome); saveErr != nil {
		r.logError(job, decision, "record_outcome", saveErr)
		return wrapRecordOutcomeError(outcome, saveErr)
	}
	r.logCompletionOutcome(runCompletion{
		job:      job,
		decision: decision,
		outcome:  outcome,
		err:      runErr,
		pid:      handle.PID(),
		timedOut: timedOut,
	})
	if runErr != nil {
		r.logError(job, decision, "execute_command", runErr)
	}
	return runErr
}

func classifyExecutionResult(ctx context.Context, err error) (string, error, bool) {
	if err != nil {
		return OutcomeSkipped, fmt.Errorf("execute command: %w", err), false
	}
	if ctx.Err() == context.Canceled {
		return OutcomeSkipped, nil, false
	}
	if ctx.Err() == context.DeadlineExceeded {
		return OutcomeExecuted, nil, true
	}
	return OutcomeExecuted, nil, false
}

func reconcileStartupState(store StateStore, cfg JobConfig, st JobState, logWriter io.Writer) (bool, JobState, error) {
	if st.ActiveExecution == nil {
		return false, st, nil
	}
	if pidRunning(st.ActiveExecution.PID) {
		return true, st, nil
	}

	active := st.ActiveExecution
	st = clearRecoveredExecution(st)
	if err := store.Save(st); err != nil {
		writeLogEvent(logWriter, time.Now().UTC(), "ERROR", "executor", "error",
			logField{key: "job", value: cfg.Name},
			logField{key: "identity", value: cfg.Identity},
			logField{key: "period", value: activePeriodID(active)},
			logField{key: "operation", value: "reconcile_recovered_execution"},
			logField{key: "error", value: err.Error()},
		)
		return false, JobState{}, err
	}
	writeLogEvent(logWriter, time.Now().UTC(), "WARN", "policy", "skipped",
		logField{key: "job", value: cfg.Name},
		logField{key: "identity", value: cfg.Identity},
		logField{key: "period", value: st.LastHandledPeriodID},
		logField{key: "nominal", value: st.LastNominalTime},
		logField{key: "chosen", value: st.LastChosenTime},
		logField{key: "pid", value: pidOrNil(active.PID)},
		logField{key: "reason", value: "recovered_stale_active"},
	)
	return false, st, nil
}

func (r *runtime) reconcileRecoveredActive(job *runtimeJob) error {
	if !job.recoveredActive || job.state.ActiveExecution == nil {
		return nil
	}
	if pidRunning(job.state.ActiveExecution.PID) {
		return nil
	}

	job.state = clearRecoveredExecution(job.state)
	job.recoveredActive = false
	if err := r.store.Save(job.state); err != nil {
		r.logError(job, decisionFromState(job), "reconcile_recovered_execution", err)
		return err
	}
	r.logSkipped(job, decisionFromState(job), "recovered_stale_active", activePID(job))
	return nil
}

func clearRecoveredExecution(st JobState) JobState {
	if st.ActiveExecution == nil {
		return st
	}

	active := st.ActiveExecution
	if st.LastHandledPeriodID == "" || active.PeriodID > st.LastHandledPeriodID {
		st.LastHandledPeriodID = active.PeriodID
		st.LastOutcome = OutcomeSkipped
		st.LastChosenTime = active.ChosenTime
		st.LastNominalTime = active.NominalTime
	}
	st.ActiveExecution = nil
	return st
}

func (r *runtime) logCompletionOutcome(completion runCompletion) {
	switch completion.outcome {
	case OutcomeExecuted:
		r.logExecuted(completion.job, completion.decision, completion.pid, completion.timedOut)
	case OutcomeSkipped:
		if completion.err != nil {
			r.logSkipped(completion.job, completion.decision, "execution_failed", completion.pid)
		}
	case OutcomeMissed:
		r.logMissed(completion.job, completion.decision, time.Now().UTC())
	case OutcomeUnsched:
		r.logUnschedulable(completion.job, completion.decision)
	}
}

func (r *runtime) logExecuted(job *runtimeJob, decision core.Decision, pid int, timedOut bool) {
	writeLogEvent(r.logWriter, time.Now().UTC(), "INFO", "executor", "executed",
		logField{key: "job", value: job.cfg.Name},
		logField{key: "identity", value: job.cfg.Identity},
		logField{key: "period", value: decision.PeriodID},
		logField{key: "nominal", value: decision.PeriodStart.UTC().Format(time.RFC3339)},
		logField{key: "chosen", value: decision.ChosenTime.UTC().Format(time.RFC3339)},
		logField{key: "pid", value: pidOrNil(pid)},
		logField{key: "timed_out", value: boolOrNil(timedOut)},
	)
}

func (r *runtime) logSkipped(job *runtimeJob, decision core.Decision, reason string, pid int) {
	writeLogEvent(r.logWriter, time.Now().UTC(), "WARN", "policy", "skipped",
		logField{key: "job", value: job.cfg.Name},
		logField{key: "identity", value: job.cfg.Identity},
		logField{key: "period", value: decision.PeriodID},
		logField{key: "nominal", value: decision.PeriodStart.UTC().Format(time.RFC3339)},
		logField{key: "chosen", value: chosenValue(decision)},
		logField{key: "pid", value: pidOrNil(pid)},
		logField{key: "reason", value: reason},
	)
}

func (r *runtime) logMissed(job *runtimeJob, decision core.Decision, now time.Time) {
	writeLogEvent(r.logWriter, now.UTC(), "WARN", "policy", "missed",
		logField{key: "job", value: job.cfg.Name},
		logField{key: "identity", value: job.cfg.Identity},
		logField{key: "period", value: decision.PeriodID},
		logField{key: "nominal", value: decision.PeriodStart.UTC().Format(time.RFC3339)},
		logField{key: "chosen", value: chosenValue(decision)},
		logField{key: "now", value: now.UTC().Format(time.RFC3339)},
	)
}

func (r *runtime) logUnschedulable(job *runtimeJob, decision core.Decision) {
	writeLogEvent(r.logWriter, time.Now().UTC(), "WARN", "scheduler", "unschedulable",
		logField{key: "job", value: job.cfg.Name},
		logField{key: "identity", value: job.cfg.Identity},
		logField{key: "period", value: decision.PeriodID},
		logField{key: "nominal", value: decision.PeriodStart.UTC().Format(time.RFC3339)},
		logField{key: "reason", value: "no_valid_candidate"},
	)
}

func (r *runtime) logError(job *runtimeJob, decision core.Decision, operation string, err error) {
	writeLogEvent(r.logWriter, time.Now().UTC(), "ERROR", "executor", "error",
		logField{key: "job", value: job.cfg.Name},
		logField{key: "identity", value: job.cfg.Identity},
		logField{key: "period", value: decision.PeriodID},
		logField{key: "operation", value: operation},
		logField{key: "error", value: err.Error()},
	)
}

func chosenValue(decision core.Decision) any {
	if decision.ChosenTime.IsZero() {
		return nil
	}
	return decision.ChosenTime.UTC().Format(time.RFC3339)
}

func pidOrNil(pid int) any {
	if pid <= 0 {
		return nil
	}
	return pid
}

func boolOrNil(v bool) any {
	if !v {
		return nil
	}
	return v
}

func activePID(job *runtimeJob) int {
	if job.state.ActiveExecution == nil {
		return 0
	}
	return job.state.ActiveExecution.PID
}

func activePeriodID(active *ActiveExecutionState) string {
	if active == nil {
		return ""
	}
	return active.PeriodID
}

func decisionFromState(job *runtimeJob) core.Decision {
	if job.state.ActiveExecution == nil {
		return core.Decision{}
	}
	decision := core.Decision{
		PeriodID: job.state.ActiveExecution.PeriodID,
	}
	if nominal := job.state.ActiveExecution.NominalTime; nominal != "" {
		if parsed, err := time.Parse(time.RFC3339, nominal); err == nil {
			decision.PeriodStart = parsed
		}
	}
	if chosen := job.state.ActiveExecution.ChosenTime; chosen != "" {
		if parsed, err := time.Parse(time.RFC3339, chosen); err == nil {
			decision.ChosenTime = parsed
		}
	}
	return decision
}
