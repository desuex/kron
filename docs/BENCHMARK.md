# BENCHMARK.md

## Purpose

Define measurable performance and reliability boundaries for `krond` before broader cron-drop-in rollout.

This document is release-oriented: if benchmark gates fail, release is blocked until either:

- the regression is fixed, or
- the gate is explicitly re-baselined with rationale.

---

## Scope

Benchmark coverage for daemon workflows includes:

- CPU and memory overhead of scheduler/runtime loop
- Dispatch delay and end-to-end timing drift
- Job execution throughput under controlled load
- State write overhead and restart behavior
- Signal handling and shutdown latency
- Zombie-process safety and child reaping behavior
- Memory growth over sustained runtime (leak screening)

Out of scope for this phase:

- Kubernetes operator performance (`kron-operator`)
- Full host-level cron parity performance
- Kernel-level profiling and eBPF tracing

---

## Test Profiles

Use three fixed workload profiles to keep results comparable across commits.

| Profile | Jobs | Schedule Pattern | Command Pattern | Target Use |
|---|---:|---|---|---|
| `S` | 100 | spread over 1-5 minute windows | `/bin/true` | baseline regression checks |
| `M` | 1,000 | mixed windows, mixed constraints | `/bin/sh -c "sleep 0.02"` | realistic host load |
| `L` | 5,000 | dense periods, pessimistic constraints | `/bin/sh -c "sleep 0.05"` | stress and failure discovery |

All profile generators must be deterministic from a fixed seed.

---

## Acceptance Gates (Alpha)

Unless otherwise stated, gates are evaluated on profile `M` on an idle CI-equivalent Linux runner.

| Area | Metric | Gate |
|---|---|---|
| Scheduler cost | `runtime.Step()` wall time, p95 | `<= 120 ms` |
| Scheduler cost | `runtime.Step()` wall time, p99 | `<= 250 ms` |
| Dispatch delay | `actual_start - chosen_time`, p95 | `<= 250 ms` |
| Dispatch delay | `actual_start - chosen_time`, p99 | `<= 1000 ms` |
| Timing accuracy | `abs(actual_start - chosen_time)` under no host pressure, p95 | `<= 150 ms` |
| CPU overhead | idle daemon CPU on profile `S` | `<= 5%` of one core |
| Memory footprint | steady-state RSS on profile `M` | `<= 150 MiB` |
| Leak screen | RSS growth over 60 min steady run | `<= 5 MiB` net growth |
| Shutdown | `SIGTERM` to clean exit | `<= 5 s` |
| Zombie safety | unreaped children after test completion | `0` |

Notes:

- Dispatch and timing gates include scheduler tick effects.
- If host noise is high, rerun 3 times and use median of p95/p99 values.

---

## Measurement Method

### Environment

- Dedicated benchmark runner (or isolated VM) with pinned CPU quota.
- NTP-synchronized clock.
- No co-located heavy jobs during benchmark window.
- Same Go toolchain and build flags across comparison runs.

### Execution Rules

- Run each profile at least 3 times.
- Keep config and seed fixed for each profile.
- Record:
  - commit SHA
  - platform and kernel
  - Go version
  - profile (`S`, `M`, `L`)
  - all percentile outputs and raw summary stats

### Reporting Format

Each run should emit a machine-readable summary (`json`) and a human summary (`md`/log) with:

- pass/fail for every gate
- percent delta vs previous accepted baseline
- top contributors when regressions are detected

---

## Required Benchmark Scenarios

### 1. Scheduler Throughput

Measure runtime loop cost with no command contention and with controlled command contention.

- Input: profile `S`, `M`, `L`
- Output: `Step()` p50/p95/p99 and max
- Failure condition: p95/p99 gate breach

### 2. Dispatch and Timing Drift

Measure how close execution start is to deterministic `chosen_time`.

- Track `chosen_time`, process spawn timestamp, and observed command start time
- Report absolute drift and signed delay distributions
- Validate p95/p99 delay gates

### 3. State I/O Overhead

Measure cost of per-job atomic state writes during execute/skip/miss flows.

- Compare fs-backed state dir on local SSD
- Track write latency percentiles and error counts
- Flag regressions that increase p95 write cost by >20% vs baseline

### 4. Long-Run Leak Screening

Run profile `M` for 60 minutes.

- Sample RSS every 10 seconds
- Collect Go heap stats at fixed intervals
- Pass only if net RSS growth is within gate and no monotonic unbounded trend is detected

### 5. Signal Handling and Shutdown

Verify daemon behavior for `SIGTERM` and `SIGINT`.

- Measure signal-to-exit latency
- Confirm in-flight child handling follows policy
- Confirm state files remain valid JSON and restart-safe

### 6. Zombie Process Safety

Induce rapid child exits and forced cancellations.

- After workload and shutdown, scan process table for defunct children owned by daemon parent
- Pass only with zero zombies

---

## Reliability Regression Checks

The following are mandatory on release candidate branches:

- No duplicate execution for the same `periodID` across restart tests
- No state corruption after forced termination tests
- No deadlock/hang during repeated signal storms (`SIGTERM` then `SIGINT`)
- No unbounded catch-up loop beyond configured caps

---

## Current Status (2026-02-25)

- `krond` is an early usable slice with deterministic scheduling, synchronous execution, and atomic per-job state.
- A formal benchmark harness is not yet wired into CI.
- This document defines the gates and scenarios to implement for the cron-drop-in stage.

Implementation backlog for this benchmark program should be tracked under milestone work items and enforced before broader production claims.
