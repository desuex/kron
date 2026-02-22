# STATE.md

## Scope

This document defines the persistent state model for Kron adapters that execute jobs directly (e.g., `krond`).

`kron-core` is stateless and does not persist state.

This specification ensures:

* Idempotency (at most one execution per period)
* Crash safety
* Upgrade compatibility
* Deterministic behavior across restarts

---

## Goals

State persistence must guarantee:

1. No duplicate execution for the same `(identity, period_id)`.
2. No unbounded catch-up after restart.
3. Crash-safe writes.
4. Backward-compatible upgrades within a major version.

---

## Storage Location

Default base directory:

```id="n1u7fa"
/var/lib/krond/
```

Each job identity maps to one state file.

File name:

```id="p3k4lo"
<hex(sha256(identity))>.json
```

No identity string is used directly as filename.

---

## State File Schema

Each state file contains a single JSON object.

```id="e8t4qf"
State {
  Version              string
  Identity             string
  LastHandledPeriodID  string
  LastOutcome          string
  LastChosenTime       string
  LastNominalTime      string
  ActiveExecution      *ActiveState
  History              []HistoryEntry
}
```

All timestamps are RFC3339 UTC strings.

---

## Field Definitions

### Version

Semantic version of the state schema.

Format:

```id="x8j9qs"
"1"
```

Version changes require explicit migration logic.

---

### Identity

Stable string used for seed derivation and logging.

Must match the identity provided to the scheduler.

---

### LastHandledPeriodID

Period ID of the most recently handled period.

If empty, no period has been handled yet.

---

### LastOutcome

One of:

* `executed`
* `skipped`
* `missed`
* `unschedulable`

Represents the terminal outcome for `LastHandledPeriodID`.

---

### LastChosenTime

UTC timestamp of the chosen time for `LastHandledPeriodID`.

Empty if unschedulable.

---

### LastNominalTime

UTC timestamp of the nominal time for `LastHandledPeriodID`.

---

### ActiveExecution

Present only when an execution is currently running.

```id="g0m2rj"
ActiveState {
  PeriodID     string
  PID          int
  StartedAt    string
  ChosenTime   string
}
```

`PID` must be verified at daemon restart.

---

### History

Optional bounded history for observability.

```id="z6c4ra"
HistoryEntry {
  PeriodID    string
  Outcome     string
  NominalTime string
  ChosenTime  string
  CompletedAt string
  ExitCode    *int
}
```

History length is capped by configuration.

Oldest entries are dropped first.

---

## State Transitions

State changes occur only on:

* Decision finalization
* Execution start
* Execution completion
* Skip
* Missed
* Unschedulable
* Crash recovery reconciliation

Transitions are atomic.

---

## Write Semantics

All state writes must be atomic.

Procedure:

1. Serialize state to memory.
2. Write to temporary file in same directory.
3. `fsync()` temporary file.
4. `rename()` to final filename.
5. `fsync()` directory.

Partial writes must never replace valid state.

---

## Idempotency Rules

Before executing a period:

1. Load state.
2. If `LastHandledPeriodID == current PeriodID`:

   * Do not execute.
3. If `ActiveExecution.PeriodID == current PeriodID`:

   * Do not execute.

After execution begins:

* Update `ActiveExecution`.
* Persist immediately.

After terminal outcome:

* Update:

  * `LastHandledPeriodID`
  * `LastOutcome`
  * `LastChosenTime`
  * `LastNominalTime`
* Clear `ActiveExecution`
* Append to `History`
* Persist

---

## Crash Recovery

On startup:

1. Load state file.
2. If `ActiveExecution` exists:

   * Check if PID exists.
   * If PID exists:

     * Treat as active.
   * If PID does not exist:

     * Mark as completed with unknown exit code.
     * Clear `ActiveExecution`.
     * Persist correction.

Kron must never assume execution succeeded unless explicitly observed.

---

## Period Advancement Rules

At evaluation time:

If `LastHandledPeriodID` equals current period:

* Skip execution.

If `LastHandledPeriodID` is earlier than current period:

* Evaluate current period only.
* Do not backfill multiple historical periods.

State must prevent multi-period replay after downtime.

---

## Deadline Interaction

If period is missed due to deadline:

* Record:

  * `LastHandledPeriodID`
  * `LastOutcome = "missed"`
  * `LastChosenTime`
  * `LastNominalTime`
* Persist immediately.

Missed periods must not be retried.

---

## Unschedulable Period

If no candidate found:

* Record:

  * `LastHandledPeriodID`
  * `LastOutcome = "unschedulable"`
  * `LastChosenTime = ""`
  * `LastNominalTime`
* Persist immediately.

---

## Concurrency Interaction

If `concurrency=forbid` and active execution exists:

* Record `LastHandledPeriodID`
* `LastOutcome = "skipped"`
* Persist

If `concurrency=replace`:

* Replace active process.
* Update `ActiveExecution`
* Persist

---

## State Corruption Handling

If state file:

* Does not exist:

  * Initialize new state.
* Is unreadable JSON:

  * Move corrupted file to:

    ```
    <filename>.corrupt.<timestamp>
    ```
  * Initialize new state.
* Has incompatible `Version`:

  * Attempt migration.
  * If migration fails:

    * Refuse to start.

---

## Migration Rules

State version increments require:

* Explicit migration function.
* Deterministic transformation.
* No loss of `LastHandledPeriodID`.
* Preservation of idempotency guarantees.

Migration must occur before scheduling begins.

---

## File Permissions

State files must be created with:

```id="r8sz3f"
0600
```

State directory must be:

```id="v2pkq1"
0700
```

Permissions must prevent unauthorized modification.

---

## Locking

Single-daemon enforcement:

* Acquire exclusive lock on state directory or PID file.
* Lock must persist for daemon lifetime.
* Failure to acquire lock results in immediate exit.

State file writes must not rely on file locks for atomicity.

---

## Invariants

State persistence guarantees:

1. At most one execution per `(identity, period_id)`.
2. No duplicate execution after crash.
3. No replay of already handled periods.
4. Terminal outcomes are permanent.
5. Active execution is recoverable after restart.
6. Corrupted state does not cause duplicate execution.
7. History retention does not affect scheduling correctness.

---

## Non-Goals

State layer does not:

* Store distribution math internals.
* Store PRNG internal state.
* Persist future decisions.
* Persist entire schedule history.

State persists only what is required to enforce correctness and idempotency.
