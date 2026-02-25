package daemon

import (
	"context"
	"fmt"
	"testing"
	"time"

	"kron/core/pkg/core"
)

type benchmarkStateStore struct {
	states map[string]JobState
}

func newBenchmarkStateStore() *benchmarkStateStore {
	return &benchmarkStateStore{states: map[string]JobState{}}
}

func (s *benchmarkStateStore) Load(identity string) (JobState, error) {
	if st, ok := s.states[identity]; ok {
		return st, nil
	}
	return JobState{
		Version:  stateVersion,
		Identity: identity,
	}, nil
}

func (s *benchmarkStateStore) Save(state JobState) error {
	s.states[state.Identity] = state
	return nil
}

type noopExecutor struct{}

func (noopExecutor) Run(context.Context, CommandSpec) (int, error) {
	return 0, nil
}

func BenchmarkRuntimeStepNoDue(b *testing.B) {
	benchmarkRuntimeStep(b, false)
}

func BenchmarkRuntimeStepDue(b *testing.B) {
	benchmarkRuntimeStep(b, true)
}

func BenchmarkRuntimeStepForbidBusy(b *testing.B) {
	cases := []struct {
		name string
		jobs int
	}{
		{name: "S100", jobs: 100},
		{name: "M1000", jobs: 1000},
		{name: "L5000", jobs: 5000},
	}

	for _, tc := range cases {
		tc := tc
		b.Run(fmt.Sprintf("%s_ForbidBusy", tc.name), func(b *testing.B) {
			spec, err := parseCronSpec([5]string{"*", "*", "*", "*", "*"}, "UTC")
			if err != nil {
				b.Fatalf("parse cron spec: %v", err)
			}

			jobs := makeBenchmarkJobs(spec, tc.jobs)
			for i := range jobs {
				jobs[i].Policy.Concurrency = "forbid"
			}
			now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)

			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				store := newBenchmarkStateStore()
				rt, err := newRuntime(jobs, store, noopExecutor{}, now)
				if err != nil {
					b.Fatalf("newRuntime: %v", err)
				}
				for _, job := range rt.jobs {
					job.running = 1
				}
				b.StartTimer()
				if err := rt.Step(context.Background(), now); err != nil {
					b.Fatalf("Step: %v", err)
				}
			}
		})
	}
}

func benchmarkRuntimeStep(b *testing.B, due bool) {
	b.Helper()

	cases := []struct {
		name string
		jobs int
	}{
		{name: "S100", jobs: 100},
		{name: "M1000", jobs: 1000},
		{name: "L5000", jobs: 5000},
	}
	mode := "NoDue"
	if due {
		mode = "Due"
	}

	for _, tc := range cases {
		tc := tc
		b.Run(fmt.Sprintf("%s_%s", tc.name, mode), func(b *testing.B) {
			spec, err := parseCronSpec([5]string{"*", "*", "*", "*", "*"}, "UTC")
			if err != nil {
				b.Fatalf("parse cron spec: %v", err)
			}

			jobs := makeBenchmarkJobs(spec, tc.jobs)
			now := time.Date(2026, 3, 1, 0, 0, 30, 0, time.UTC)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				store := newBenchmarkStateStore()
				if !due {
					markNotDue(store, jobs, now)
				}
				rt, err := newRuntime(jobs, store, noopExecutor{}, now)
				if err != nil {
					b.Fatalf("newRuntime: %v", err)
				}
				b.StartTimer()
				if err := rt.Step(context.Background(), now); err != nil {
					b.Fatalf("Step: %v", err)
				}
			}
		})
	}
}

func makeBenchmarkJobs(spec CronSpec, count int) []JobConfig {
	jobs := make([]JobConfig, 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("/etc/krond.d/bench.kron:job-%d", i)
		jobs = append(jobs, JobConfig{
			Identity:  id,
			Name:      fmt.Sprintf("job-%d", i),
			Schedule:  spec,
			Command:   CommandSpec{Raw: "/bin/true"},
			Window:    0,
			Mode:      core.WindowModeAfter,
			Dist:      core.DistributionUniform,
			Timezone:  "UTC",
			Seed:      core.SeedStrategyStable,
			Policy:    PolicySpec{Concurrency: DefaultConcurrency},
			SkewShape: 0,
		})
	}
	return jobs
}

func markNotDue(store *benchmarkStateStore, jobs []JobConfig, now time.Time) {
	nominal := now.UTC().Truncate(time.Minute).Format(time.RFC3339)
	for _, job := range jobs {
		store.states[job.Identity] = JobState{
			Version:             stateVersion,
			Identity:            job.Identity,
			LastHandledPeriodID: nominal,
			LastNominalTime:     nominal,
			LastOutcome:         OutcomeExecuted,
		}
	}
}

func BenchmarkFileStateStoreSave(b *testing.B) {
	store := FileStateStore{Dir: b.TempDir()}
	state := JobState{
		Version:             stateVersion,
		Identity:            "/etc/krond.d/bench.kron:io-save",
		LastHandledPeriodID: "2026-03-01T00:00:00Z",
		LastOutcome:         OutcomeExecuted,
		LastChosenTime:      "2026-03-01T00:00:00Z",
		LastNominalTime:     "2026-03-01T00:00:00Z",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.Save(state); err != nil {
			b.Fatalf("Save: %v", err)
		}
	}
}

func BenchmarkFileStateStoreLoad(b *testing.B) {
	store := FileStateStore{Dir: b.TempDir()}
	const identity = "/etc/krond.d/bench.kron:io-load"
	if err := store.Save(JobState{
		Version:             stateVersion,
		Identity:            identity,
		LastHandledPeriodID: "2026-03-01T00:00:00Z",
		LastOutcome:         OutcomeExecuted,
		LastChosenTime:      "2026-03-01T00:00:00Z",
		LastNominalTime:     "2026-03-01T00:00:00Z",
	}); err != nil {
		b.Fatalf("seed save: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Load(identity); err != nil {
			b.Fatalf("Load: %v", err)
		}
	}
}
