# ERROR-MODEL.md

## Scope

This document defines Kron’s error model.

It specifies:

* Error categories
* Error propagation rules
* Handling semantics
* Logging requirements
* Failure guarantees

This applies to:

* `kron-core`
* `krond`
* `kron-operator`

---

## Design Principles

1. Errors must be explicit.
2. Errors must never cause duplicate execution.
3. Errors must never cause unbounded replay.
4. Determinism must not be compromised by errors.
5. Invalid configuration must fail fast.
6. Runtime failures must degrade safely.

---

## Error Categories

Errors are classified as:

* `ConfigurationError`
* `ValidationError`
* `SchedulingError`
* `ConstraintError`
* `ExecutionError`
* `PersistenceError`
* `SystemError`
* `IncompatibleStateError`

Each category has defined behavior.

---

## ConfigurationError

Raised when:

* Syntax is invalid.
* Required fields missing.
* Unknown distribution.
* Unknown seed strategy.
* Invalid parameters.
* Invalid timezone.
* Negative durations.

### Behavior

* In `krond`: daemon refuses to start.
* In `kron-operator`: resource marked invalid; reconciliation stops for that resource.
* In `kron-core`: error returned, no decision produced.

No scheduling must occur for invalid configuration.

---

## ValidationError

Raised when configuration is syntactically valid but semantically invalid.

Examples:

* Window duration too large.
* Constraint clauses malformed.
* Distribution parameters out of allowed range.

### Behavior

Same as `ConfigurationError`.

---

## SchedulingError

Raised during decision computation when engine cannot proceed.

Examples:

* Internal invariant violation.
* PRNG initialization failure.
* Hash computation failure.

### Behavior

* `kron-core` returns error.
* Adapter logs error.
* No execution occurs.
* Period is not marked handled.
* Retry allowed on next reconciliation or loop iteration.

Scheduling errors must not advance period state.

---

## ConstraintError

Raised when:

* Constraint evaluation fails due to malformed clause.
* Timezone resolution fails during constraint evaluation.

### Behavior

* Treated as `ValidationError` if static.
* Treated as `SchedulingError` if dynamic.
* No execution occurs.

---

## Unschedulable Condition

Not an error.

Occurs when:

* Valid configuration
* Valid scheduling
* No candidate satisfies constraints within sampling budget

### Behavior

* Period outcome is `unschedulable`.
* Period is marked handled.
* No retry.
* Logged at `WARN`.

---

## ExecutionError

Occurs when:

* Fork fails.
* Exec fails.
* Permission drop fails.
* Command not found.
* Process exits with non-zero code.

### Behavior

* If fork/exec fails before process starts:

  * Period outcome is `executed` only if process was created.
  * If process was not created, treat as `missed` only if deadline exceeded.
  * Otherwise log error and do not mark handled until explicit outcome determined.

* If process exits non-zero:

  * Period outcome is `executed`.
  * Exit code recorded.
  * No automatic retry.

Execution failures do not trigger retries unless explicitly implemented in future versions.

---

## PersistenceError

Occurs when:

* State file write fails.
* fsync fails.
* Rename fails.
* State file unreadable.
* Migration fails.

### Behavior

Before execution:

* If state cannot be read safely:

  * Daemon must refuse to start.

After execution begins:

* If state write fails after marking execution started:

  * Process must be terminated.
  * Fatal error.
  * Daemon exits.

After terminal outcome:

* If state write fails:

  * Fatal error.
  * Daemon exits.

Persistence integrity is mandatory for idempotency.

---

## SystemError

Occurs when:

* PID verification fails.
* OS-level resource exhaustion.
* File descriptor exhaustion.
* Lock acquisition fails.

### Behavior

* If critical to correctness:

  * Fatal.
  * Daemon exits.
* If transient:

  * Log error.
  * Retry with backoff.
  * Do not mark period handled.

System errors must never silently skip execution.

---

## IncompatibleStateError

Occurs when:

* State version unsupported.
* Migration fails.
* Required fields missing in state file.

### Behavior

* Daemon refuses to start.
* No scheduling occurs.
* Explicit log at `ERROR`.

---

## Deadline Interaction

If evaluation occurs after deadline:

* Period outcome is `missed`.
* This is not an error.
* Logged at `INFO`.

Deadline expiration must not be treated as failure.

---

## Concurrency Conflicts

If:

* `forbid` and active execution exists:

  * Period outcome is `skipped`.
  * Not an error.

If:

* `replace` and termination fails:

  * Log `ERROR`.
  * Do not start new execution.
  * Period remains unhandled.
  * Retry permitted until deadline exceeded.

---

## Clock Anomalies

If system clock jumps forward:

* Evaluate current period.
* Apply deadline rules.
* No error.

If system clock jumps backward:

* Already handled periods must not re-execute.
* No error.
* Logged at `WARN`.

Clock changes are not treated as failures.

---

## Partial Failure Handling

If:

* Decision computed successfully
* Trigger attempted
* State write fails

Daemon must exit immediately to prevent duplicate execution.

---

## Retry Rules

Retries are allowed only for:

* Transient scheduling errors
* Transient system errors before execution begins

Retries must:

* Not alter seed inputs
* Not alter decision
* Not generate a new chosen time

Retries must not create new periods.

---

## Error Logging Contract

Every error must log:

* `identity`
* `period_id` (if applicable)
* `component`
* `error_type`
* `operation`
* `message`

Fatal errors must log at `ERROR` level before exit.

---

## Fatal Conditions

Daemon must exit on:

* State corruption without recovery
* State write failure after execution start
* Lock acquisition failure
* Incompatible state version
* Irrecoverable persistence error

---

## Non-Fatal Conditions

Daemon must continue operation on:

* Single job configuration error (if multi-job environment)
* Execution non-zero exit code
* Missed deadlines
* Unschedulable period
* Constraint rejection

---

## Invariants Under Error

Kron guarantees:

1. No duplicate execution for same period.
2. No execution without a valid decision.
3. No execution when configuration invalid.
4. No execution beyond deadline.
5. Fatal persistence errors prevent further scheduling.
6. Unschedulable periods are terminal.
7. Execution failure does not trigger implicit retry.

---

## Adapter-Specific Notes

### krond

* Fatal persistence errors require immediate shutdown.
* State integrity is mandatory.

### kron-operator

* Errors must surface as Kubernetes Events and Conditions.
* Reconciliation must be idempotent.
* Controller must not create duplicate Jobs.

---

## Summary

Errors in Kron are:

* Categorized.
* Explicit.
* Logged.
* Deterministic in handling.
* Never allowed to violate idempotency or determinism.

Correctness and safety take precedence over availability.
