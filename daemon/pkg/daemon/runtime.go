package daemon

import (
	"context"
	"fmt"
	"os"
	"time"

	"kron/core/pkg/core"
)

const maxJobCatchupPerStep = 32

type StartOptions struct {
	ConfigPath string
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
	running     int
}

type runtimeJob struct {
	cfg           JobConfig
	state         JobState
	nextPeriod    time.Time
	decision      core.Decision
	decisionReady bool
	running       int
}

type runCompletion struct {
	job      *runtimeJob
	decision core.Decision
	outcome  string
	err      error
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

	rt, err := newRuntime(jobs, FileStateStore{Dir: opts.StateDir}, OSExecutor{}, time.Now().UTC())
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

func newRuntime(jobs []JobConfig, store StateStore, executor Executor, now time.Time) (*runtime, error) {
	completionCap := len(jobs) * maxJobCatchupPerStep
	if completionCap < 64 {
		completionCap = 64
	}

	rt := &runtime{
		jobs:        make([]*runtimeJob, 0, len(jobs)),
		store:       store,
		executor:    executor,
		completions: make(chan runCompletion, completionCap),
	}

	for _, cfg := range jobs {
		st, err := store.Load(cfg.Identity)
		if err != nil {
			return nil, fmt.Errorf("load state for %s: %w", cfg.Identity, err)
		}
		next, err := nextPeriodFromState(cfg.Schedule, st, now)
		if err != nil {
			return nil, fmt.Errorf("compute next period for %s: %w", cfg.Identity, err)
		}

		rt.jobs = append(rt.jobs, &runtimeJob{
			cfg:        cfg,
			state:      st,
			nextPeriod: next,
		})
	}
	return rt, nil
}

func nextPeriodFromState(schedule CronSpec, st JobState, now time.Time) (time.Time, error) {
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
		if job.state.LastHandledPeriodID == decision.PeriodID {
			if err := r.advancePeriod(job); err != nil {
				return err
			}
			continue
		}

		if decision.Unschedulable {
			if err := r.recordOutcome(job, decision, OutcomeUnsched); err != nil {
				return err
			}
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
				return err
			}
			if err := r.advancePeriod(job); err != nil {
				return err
			}
			continue
		}

		if job.cfg.Policy.Concurrency == "forbid" && job.running > 0 {
			if err := r.recordOutcome(job, decision, OutcomeSkipped); err != nil {
				return err
			}
			if err := r.advancePeriod(job); err != nil {
				return err
			}
			continue
		}

		r.dispatchRun(ctx, job, decision)
		if err := r.advancePeriod(job); err != nil {
			return err
		}
	}
	return nil
}

func (r *runtime) execute(ctx context.Context, job *runtimeJob, decision core.Decision) error {
	outcome, runErr := r.runCommand(ctx, job, decision)
	if saveErr := r.recordOutcome(job, decision, outcome); saveErr != nil {
		return wrapRecordOutcomeError(outcome, saveErr)
	}
	return runErr
}

func (r *runtime) runCommand(ctx context.Context, job *runtimeJob, decision core.Decision) (string, error) {
	runCtx := ctx
	cancel := func() {}
	if job.cfg.Command.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, job.cfg.Command.Timeout)
	}
	defer cancel()

	exitCode, err := r.executor.Run(runCtx, job.cfg.Command)
	if err != nil {
		return OutcomeSkipped, fmt.Errorf("execute command: %w", err)
	}
	if runCtx.Err() == context.Canceled {
		return OutcomeSkipped, nil
	}
	if runCtx.Err() == context.DeadlineExceeded {
		fmt.Fprintf(os.Stderr, "krond: timeout for job=%s period=%s exit=%d\n", job.cfg.Identity, decision.PeriodID, exitCode)
	}
	return OutcomeExecuted, nil
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

func (r *runtime) dispatchRun(ctx context.Context, job *runtimeJob, decision core.Decision) {
	job.running++
	r.running++

	go func(job *runtimeJob, decision core.Decision) {
		outcome, err := r.runCommand(ctx, job, decision)
		r.completions <- runCompletion{
			job:      job,
			decision: decision,
			outcome:  outcome,
			err:      err,
		}
	}(job, decision)
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
		return nil
	}

	if err := r.recordOutcome(completion.job, completion.decision, completion.outcome); err != nil {
		return fmt.Errorf("job %s: %w", completion.job.cfg.Identity, wrapRecordOutcomeError(completion.outcome, err))
	}
	if completion.err != nil {
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
