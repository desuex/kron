# SPEC.md

## Scope

This document specifies Kron’s scheduling and execution semantics as a stable contract.

It applies to all Kron deployments and adapters:

* `kron-core` (engine)
* `krond` (host daemon)
* `kron-operator` (Kubernetes controller)

Where an adapter cannot enforce a behavior directly, it must preserve the observable contract.

---

## Terminology

* **Schedule**: A cron expression interpreted in a timezone.
* **Timezone**: IANA timezone used to interpret schedule and calendar constraints.
* **Nominal time**: The scheduled timestamp produced by resolving a schedule for a period.
* **Period**: The discrete scheduling opportunity anchored at one nominal time.
* **Period ID**: Canonical identifier for a period.
* **Window**: The allowed time interval in which an execution may be chosen.
* **Chosen time**: The specific timestamp selected within the window for a period.
* **Decision**: The computed record containing period, window, distribution, seed, and chosen time.
* **Constraint**: A rule restricting which timestamps are allowed.
* **Candidate**: A sampled timestamp within the window prior to constraint validation.
* **Deadline**: Maximum allowed lateness relative to chosen time to still execute.
* **Execution**: One realized run of a job (process spawn or Kubernetes `Job` creation).
* **Handled period**: A period for which Kron has reached a terminal outcome.
* **Terminal outcome**: `executed`, `skipped`, `missed`, or `unschedulable`.
* **Active execution**: An execution that has started but not completed.
* **Identity**: Stable identifier of a schedule entry.

---

## Time Model

* All internal comparisons use UTC instants.
* Input schedules and constraints are interpreted in the configured timezone.
* All persisted timestamps are stored as RFC3339 UTC.
* Kron’s decisions are defined at second-level granularity for chosen timestamps unless a platform supports higher precision; implementations may use higher precision but must not violate determinism for the same inputs.

---

## Inputs

A job definition provides:

* `identity`: stable identifier
* `schedule`: cron expression
* `timezone`: optional, default `UTC`
* `window_mode`: `after` or `around`
* `window_duration`: non-negative duration
* `distribution`: name + parameters
* `seed_strategy`: name + optional parameters
* `salt`: optional string
* `constraints`: optional `only` and/or `avoid`
* `policy`:

  * `concurrency`: `allow|forbid|replace`
  * `deadline`: duration (default `0s`)
  * `suspend`: boolean (default `false`)

Adapters also provide:

* `now`: current time instant

---

## Outputs

For each job, Kron produces:

* A `Decision` for a specific `Period`:

  * `period_id`
  * `nominal_time`
  * `window_start`
  * `window_end`
  * `chosen_time`
  * `timezone`
  * `distribution` + parameters
  * `seed_strategy` + parameters
  * `seed_hash`
  * `constraints_applied` summary
* A terminal outcome per period:

  * `executed`
  * `skipped`
  * `missed`
  * `unschedulable`

---

## Periods

### Period definition

A period is the interval of responsibility anchored at one resolved nominal time.

For a schedule `S`, timezone `TZ`, and an evaluation time `t`, define:

* `prev_nominal(t)`: greatest nominal time ≤ `t`
* `next_nominal(t)`: smallest nominal time > `t`

A period is identified by its nominal time.

### Period ID

`period_id` is the RFC3339 UTC representation of the nominal time.

Canonical form:

```
period_id = nominal_time_utc_rfc3339
```

Implementations must treat the same nominal instant as the same period even if represented in different timezones.

---

## Window Semantics

Given nominal time `N` and window duration `D`:

* If `window_mode=after`:

  * `window_start = N`
  * `window_end = N + D`
* If `window_mode=around`:

  * `window_start = N - (D / 2)`
  * `window_end = N + (D / 2)`

`window_start` and `window_end` are inclusive bounds for candidate generation, with the constraint that the chosen time must satisfy:

```
window_start ≤ chosen_time ≤ window_end
```

If `D=0`, then:

```
chosen_time = N
```

---

## Distribution Semantics

A distribution maps a seeded pseudorandom stream to a time within the window.

Distributions must be bounded: no value outside the window may be chosen.

Distributions may require parameters; missing parameters imply deterministic defaults.

Distribution evaluation must be deterministic for identical inputs.

The chosen time must be stable for:

* the same `identity`
* the same `period_id`
* the same schedule configuration
* the same seed strategy and salt

---

## Seed Semantics

### Determinism requirement

Kron must produce the same `chosen_time` for the same job definition and period, independent of:

* process restarts
* leader changes
* reconciliation frequency

### Seed inputs

Seed derivation uses:

* `identity`
* `period_key` (derived from `period_id` and seed strategy)
* `salt` (string, possibly empty)

### Seed strategies

Seed strategies define the `period_key`:

* `stable`: `period_key = period_id`
* `daily`: `period_key = YYYY-MM-DD in TZ corresponding to nominal time`
* `weekly`: `period_key = ISO week (YYYY-Www) in TZ corresponding to nominal time`

### Seed hash

Kron computes:

```
seed_hash = HASH(identity || "\n" || period_key || "\n" || salt)
```

Where:

* `HASH` is a stable algorithm selected by Kron (cryptographic hash required)
* `identity`, `period_key`, and `salt` are UTF-8 strings
* `||` indicates concatenation

`seed_hash` is the canonical seed representation exposed in logs and status.

The pseudorandom generator uses `seed_hash` as seed material in a stable, specified transformation.

---

## Constraint Semantics

Constraints restrict allowable chosen times.

* `only` defines the allowed set.
* `avoid` defines the disallowed set.
* If both are present, a time is valid only if it satisfies `only` and does not satisfy `avoid`.

Constraints are evaluated in the schedule timezone.

Constraints apply to candidate timestamps during decision computation.

### Candidate selection with constraints

To compute `chosen_time`:

1. Sample a candidate within the window using the distribution.
2. If candidate violates constraints, reject and sample a new candidate.
3. Continue until a valid candidate is found or the sampling budget is exhausted.

Sampling budget is deterministic and fixed by Kron.

### Unschedulable periods

If no valid candidate is found within the sampling budget, the period outcome is `unschedulable` and no execution occurs.

---

## Decision Computation

For each job and period:

1. Resolve `nominal_time` for the period based on schedule and timezone.
2. Compute window bounds.
3. Compute `period_key` based on seed strategy.
4. Compute `seed_hash`.
5. Initialize deterministic pseudorandom generator from `seed_hash`.
6. Sample candidate(s) according to distribution within window.
7. Apply constraints.
8. Produce `Decision` including chosen time and metadata.

A `Decision` is specific to one period.

If the job definition changes, decisions for future periods may change. Decisions for already-handled periods must not cause additional execution.

---

## Handling and Outcomes

A period is handled exactly once by reaching one terminal outcome.

Terminal outcomes:

* `executed`: an execution was started for the period
* `skipped`: execution was intentionally not started due to policy
* `missed`: execution was not started because the deadline expired
* `unschedulable`: no valid time could be selected within the window and constraints

Once a period is handled, Kron must not start another execution for that period.

---

## Execution Trigger Semantics

A period becomes eligible for trigger when:

```
now ≥ chosen_time
```

If `policy.suspend=true`, no trigger occurs and no period is handled while suspended.

When eligible, Kron attempts to reach a terminal outcome by evaluating policies and system state.

---

## Deadline Semantics

Let:

* `C` be `chosen_time`
* `DL` be `policy.deadline`
* `now` current time at trigger evaluation

If `DL=0s`, then the period is handled as `missed` if `now > C`.

If `DL>0s`, then:

* If `now ≤ C + DL`, execution may proceed
* If `now > C + DL`, the period is handled as `missed`

---

## Concurrency Semantics

Concurrency policy is evaluated at trigger time.

Let `active` indicate whether there is an active execution for the job identity.

* `allow`: proceed to execute regardless of `active`.
* `forbid`: if `active=true`, handle period as `skipped`.
* `replace`: if `active=true`, terminate the active execution and proceed to execute.

Termination semantics for `replace` are adapter-defined but must be best-effort and observable.

---

## Idempotency Semantics

Kron must guarantee at most one execution per `(identity, period_id)`.

Adapters must implement an idempotency check before starting execution.

* In Kubernetes mode: by checking for an existing owned Job with the period identifier.
* In daemon mode: by consulting persisted state and verifying active/existing executions.

If an execution for the period is already recorded as started or completed, the period is handled and must not be executed again.

---

## State Machine

Per job identity, each period transitions:

* `Planned`: decision computed, chosen time known
* `Eligible`: now ≥ chosen_time
* `Terminal`: one of `executed|skipped|missed|unschedulable`

Execution sub-states for `executed`:

* `Running`: started, not yet completed
* `Completed`: finished with an exit result (adapter-specific)

State transitions are monotonic.

A period cannot transition from a terminal outcome to a different terminal outcome.

---

## Spec Change Semantics

A job definition change is any change to:

* schedule
* timezone
* window mode/duration
* distribution or parameters
* constraints
* seed strategy or salt
* policy settings

Effects:

* Future decisions may change starting from the first unhandled period.
* Already-handled periods must remain handled and must not execute again.
* Active executions are governed by concurrency policy and adapter behavior.

---

## Clock Change Semantics

Kron treats `now` as authoritative input.

* If `now` jumps forward, some periods may become immediately eligible and will be handled subject to deadline and idempotency.
* If `now` jumps backward, Kron must not re-handle already handled periods.

Adapters must persist handled period outcomes to preserve idempotency across clock changes.

---

## Observability Contract

For each period and identity, Kron must expose:

* `period_id`
* `nominal_time`
* `chosen_time`
* terminal outcome
* reason for non-execution (`skipped`, `missed`, `unschedulable`) when applicable
* seed hash and distribution metadata sufficient to reproduce the decision

Logs must contain enough information to reproduce the decision off-line.

---

## Invariants

Kron guarantees:

1. `chosen_time` is within `[window_start, window_end]`.
2. At most one execution per `(identity, period_id)`.
3. Deterministic `chosen_time` for the same inputs and period.
4. No unbounded historical replay.
5. Terminal outcomes are monotonic and permanent for a period.
6. Suspension prevents executions without advancing handled periods.
7. If constraints make selection impossible, the period becomes `unschedulable` and no execution occurs.

---

## Compatibility

* The meaning of `period_id`, window computation, seed hashing inputs, and determinism rules are stable contracts.
* New distributions, parameters, and constraint types may be added.
* Any behavior change that affects decisions for the same inputs requires a major version bump of the relevant API surface.
