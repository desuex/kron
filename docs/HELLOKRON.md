# HELLOKRON.md

## What Kron is

Kron is a Kubernetes-native probabilistic scheduler.

Kron exists to schedule Kubernetes `Job` executions with controlled variability so that automation does not fire as a synchronized metronome. Kron spreads scheduled work to reduce load spikes, supports distributions that imitate human timing patterns, and enables gentle unpredictability for automation use cases.

Kron is delivered as a Kubernetes controller plus Custom Resource Definitions (CRDs). It creates ordinary Kubernetes `Job` objects. It does not require changes to Kubernetes itself.

Kron is deterministic by default: for a given resource, schedule period, and configuration, the chosen run time is reproducible and explainable.

---

## Primary goals

Kron must:

* Reduce synchronized load spikes created by many schedules firing at the same time.
* Support biased randomness so schedules can cluster early/late or around a target time.
* Preserve operational safety: no uncontrolled catch-up storms, no duplicate executions, no hidden behavior.
* Be Kubernetes-native: GitOps-friendly, observable, RBAC-minimal, compatible with common tooling.
* Provide explainability: every chosen fire time must be attributable to configuration + seed + current period.

---

## Non-goals

Kron is not:

* A general workflow engine.
* A replacement for Kubernetes `Job` semantics.
* A distributed queue or rate limiter.
* A real-time scheduler for sub-second execution.
* A tool that guarantees wall-clock execution under all cluster conditions.

---

## Architecture

Kron runs as a controller deployment in Kubernetes.

It provides one primary CRD:

* `KronJob` (name may change, semantics remain)

A `KronJob` represents a recurring schedule. The controller computes the next run time and creates a Kubernetes `Job` from a user-provided `jobTemplate` when the computed time arrives.

Kron uses leader election. At most one active controller instance makes scheduling decisions at a time.

Kron’s scheduling decisions are pure and reproducible: they depend only on the `KronJob` spec, the current schedule period, and a seed strategy.

---

## Resource model

### KronJob

A `KronJob` defines:

* A baseline schedule (cron-like).
* A window around or within which execution may occur.
* A probability distribution used to choose a specific execution time within the window.
* A deterministic seed strategy.
* Execution safety policies (concurrency, missed run behavior).
* A Kubernetes `Job` template to instantiate.

Kron creates `Job` objects with:

* An owner reference pointing to the `KronJob`.
* Labels and annotations that allow tracing decisions, identifying the period, and enforcing idempotency.
* Optional TTL and retention controlled by the `Job` template and/or Kron defaults.

---

## Scheduling semantics

### Baseline schedule

Each `KronJob` has a cron-like `schedule` that defines discrete schedule instants. Each instant defines a schedule period boundary and a nominal target time.

For each schedule instant, Kron defines one execution opportunity called a `period`.

A `period` is identified by the schedule instant timestamp in UTC plus the configured timezone context used to interpret the cron expression.

### Window

A `window` defines the allowable time range from which Kron may choose an execution time for a period.

Kron supports two window modes:

* `around`: the window is centered on the nominal schedule time.
* `after`: the window starts at the nominal schedule time and extends forward.

If not specified, the default is `after`.

A window is expressed as a duration. The effective window is:

* `around`: `[nominal - window/2, nominal + window/2]`
* `after`: `[nominal, nominal + window]`

If `window` is zero, the chosen time equals the nominal schedule time.

If the computed window start is after window end, the resource is invalid.

### Distribution

A `distribution` chooses a point in time within the window.

Kron defines these standard distributions:

* `uniform`: equal probability across the window.
* `normal`: truncated normal centered at the nominal time (or at the window midpoint for `after` mode).
* `skewEarly`: probability mass biased toward the start of the window.
* `skewLate`: probability mass biased toward the end of the window.
* `exponential`: truncated exponential producing many early (or many late) selections depending on direction.

Each distribution may accept optional parameters (e.g., standard deviation for `normal`, shape for skew distributions). All distributions are bounded to the window.

If distribution parameters are invalid, the resource is invalid.

If not specified, the default distribution is `uniform`.

### Deterministic randomness

Kron must be deterministic by default.

For each period, Kron computes a seed. The seed is derived from:

* A stable resource identity (namespace + name + UID).
* The period identifier (schedule instant timestamp).
* An optional user-provided `salt`.

Kron uses the seed to produce pseudorandom values for the distribution selection.

The chosen execution time for a given `KronJob` and period is stable across:

* Controller restarts.
* Leader changes.
* Reconciliations.

### Seed strategy

Kron supports these seed strategies:

* `stable`: uses resource identity + period + salt. Default.
* `daily`: period component is day-based in configured timezone, producing one decision per day even if multiple nominal instants exist.
* `weekly`: period component is week-based in configured timezone.
* `custom`: reserved for future extensions.

Seed strategy affects how often a new random decision is made.

### Timezones

A `KronJob` may specify a timezone for interpreting the cron schedule and for defining day/week boundaries for seed strategies.

If not specified, default is `UTC`.

Kron stores all computed timestamps in UTC in status and annotations.

---

## Execution semantics

### Triggering

Kron continually computes the next chosen run time for each `KronJob`.

When current time is greater than or equal to the chosen run time for the current period, Kron attempts to create one `Job` for that period.

Kron never creates more than one `Job` per period.

Each created `Job` is uniquely associated with a specific period via a deterministic identifier stored in labels/annotations.

### Idempotency

Job creation must be idempotent.

Before creating a `Job` for a period, Kron checks whether a `Job` with the same period identifier already exists and is owned by the `KronJob`.

If such a `Job` exists, Kron records status accordingly and does not create another.

### Concurrency policy

Kron supports:

* `Allow`: multiple Jobs may run concurrently across periods.
* `Forbid`: if a previous Job owned by the `KronJob` is still active, Kron does not start a new Job for the next period.
* `Replace`: if a previous Job is active when a new period triggers, Kron deletes the active Job and starts the new Job.

Default is `Forbid`.

### Missed periods and deadlines

Kron supports missed-run handling to prevent catch-up storms.

A `startingDeadlineSeconds` bounds how late Kron is allowed to create a `Job` for a period relative to the chosen run time.

If current time exceeds `chosenRunTime + startingDeadlineSeconds`, Kron marks the period as missed and does not create a Job for that period.

If not specified, the default behavior is to skip missed periods and only schedule the next period.

Kron never queues unlimited historical executions.

### Suspension

If `spec.suspend` is true, Kron does not create Jobs and does not advance scheduling state beyond recording that it is suspended.

When suspension is lifted, Kron schedules only future periods according to missed-period rules.

---

## Status and explainability

KronJob status includes:

* `observedGeneration`
* `lastPeriod`: identifier of the most recent period evaluated
* `lastScheduleTime`: nominal schedule time of the last period
* `lastChosenTime`: chosen execution time for the last period
* `lastJobRef`: reference to the last created Job
* `nextScheduleTime`: nominal schedule time of the next period
* `nextChosenTime`: chosen execution time for the next period
* `conditions`: readiness and error conditions

Kron writes human-readable and machine-readable decision metadata:

* Seed inputs and resulting seed hash
* Distribution name and parameters
* Window start/end
* Chosen timestamp

This metadata is stored in:

* `status`
* Job annotations on the created `Job`
* Kubernetes Events for significant actions (scheduled, skipped, missed, replaced)

---

## Observability

Kron exposes:

* Structured logs with resource identifiers and period identifiers.
* Prometheus metrics:

  * schedules evaluated
  * jobs created
  * jobs skipped due to concurrency
  * periods missed due to deadlines
  * reconciliation errors
  * time-to-decision and reconcile latency

Kron must make it easy to answer:

* When will this run next?
* Why was this time chosen?
* Why did it not run?
* Which Job corresponds to which period?

---

## Security and operational model

Kron runs with:

* Namespaced permissions when installed per-namespace.
* Optional cluster-scoped mode for multi-namespace management.

Minimum required permissions:

* Read/write `KronJob` resources.
* Create/list/watch/get `Job` resources in managed namespaces.
* Emit Events.
* Read system time and reconcile.

Kron does not execute user code. It only creates Kubernetes Jobs from user-provided templates.

---

## Compatibility and integration

Kron must be compatible with:

* GitOps workflows (Argo CD / Flux): spec-driven behavior, status-only mutations.
* Helm and Kustomize installation patterns.
* Standard Kubernetes `Job` fields and behaviors.
* PodDisruptionBudgets and cluster scheduling policies via the `jobTemplate`.

Kron does not require webhooks for core operation. Validation may be performed via CRD schema. Optional admission may be added later.

---

## Failure behavior

If the controller is down, no Jobs are created during downtime.

When the controller returns, it evaluates current time and applies missed-period rules to decide whether to schedule the current period or skip to the next.

If API writes fail, Kron retries via standard reconciliation.

No failure mode should produce uncontrolled backfills.

---

## Versioning and stability

Kron resources are versioned and may start at `v1alpha1`.

Behavioral stability is defined by:

* Determinism guarantees for stable seed strategies.
* No breaking changes to scheduling semantics without a version bump and migration path.
* Backward-compatible defaults whenever possible.

---

## User-facing contract

Given a `KronJob` spec, users can rely on:

* At most one Job per period.
* Chosen times always lie within the configured window.
* The distribution and seed strategy determine the chosen time reproducibly.
* Concurrency and missed-period policies are enforced consistently.
* Status and annotations explain scheduling decisions.

Kron’s default configuration produces:

* Deterministic spreading within an `after` window using `uniform` distribution.
* No overlapping jobs (`Forbid`).
* No catch-up storms (missed periods are skipped unless explicitly configured otherwise).
