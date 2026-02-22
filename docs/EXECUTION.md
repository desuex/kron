# EXECUTION.md

## Scope

This document defines how Kron executes scheduled work in non-Kubernetes mode (`krond`). It describes:

* Daemon lifecycle
* Job process model
* Forking and execution
* Concurrency handling
* State management
* Failure handling

This specification applies to `krond`.
Kubernetes execution is delegated to native `Job` resources and is not defined here.

---

## Process Model Overview

Kron consists of:

* `krond` — long-running daemon
* `krontab` — configuration manager (writes config files)
* Child processes — spawned per scheduled execution

`krond` is the only persistent process.

Each scheduled execution results in exactly one fork/exec cycle.

---

## krond Lifecycle

### Startup

On startup, `krond`:

1. Loads configuration files.
2. Validates all schedule syntax.
3. Loads persisted state from disk.
4. Computes the next period and chosen execution time for each entry.
5. Starts the scheduler loop.
6. Optionally writes PID file.

If configuration is invalid, `krond` exits with non-zero status.

---

## Scheduler Loop

`krond` maintains an in-memory priority queue ordered by next chosen execution time.

Main loop:

1. Determine earliest scheduled execution.
2. Sleep until that timestamp.
3. Wake up.
4. Recompute decision for that job to confirm it is still valid.
5. Attempt execution.
6. Compute next period and chosen time.
7. Reinsert into queue.

Time is always compared in UTC internally.

If system clock jumps forward, missed-period logic is applied.
If system clock jumps backward, the queue is recalculated.

---

## Job Identity

Each schedule entry has a stable identity:

```
<config_path>:<entry_name>
```

This identity is used for:

* Seed derivation
* Locking
* State files
* Logging

---

## Fork and Exec Model

When a job is triggered:

1. `krond` calls `fork()`.
2. Child process calls `execve()` with configured command.
3. Parent process:

   * Records child PID
   * Monitors exit status via `waitpid()`

No shell wrapping is performed unless explicitly configured.

Default behavior:

* Command is executed directly.
* Arguments are passed as defined.
* Environment is inherited from `krond` unless overridden.

---

## Execution Environment

Each job may define:

* `user`
* `group`
* `env`
* `cwd`
* `umask`
* `timeout`

Before `execve()`:

1. If running as root and `user` specified:

   * `setgid()`
   * `setuid()`
2. Apply `chdir()` if `cwd` defined.
3. Apply `umask()` if defined.
4. Set environment variables.

If privilege drop fails, execution is aborted.

---

## Concurrency Handling

Each job has a concurrency policy:

* `allow`
* `forbid`
* `replace`

### allow

Multiple child processes may run simultaneously.

### forbid

If a previous child process is still active:

* New execution is skipped.
* Event is logged.
* Period is marked as skipped.

### replace

If a previous child process is active:

1. Send `SIGTERM` to active PID.
2. Wait configurable grace period.
3. If still running, send `SIGKILL`.
4. Start new process.

---

## Timeout Handling

If `timeout` is defined:

1. Parent tracks execution duration.
2. When timeout expires:

   * Send `SIGTERM`.
   * Wait grace period.
   * Send `SIGKILL` if necessary.
3. Log timeout event.

Timeout applies per execution instance.

---

## State Persistence

State is stored in:

```
/var/lib/krond/<hash>.json
```

State file contains:

* Last handled period identifier
* Last chosen execution time
* Last run time
* Last exit code
* Active PID (if any)
* Concurrency state
* Seed metadata

State is written:

* After scheduling decision
* After execution completes
* After skip due to concurrency
* After missed deadline

Writes are atomic:

1. Write to temporary file.
2. fsync().
3. Rename.

---

## Missed Period Handling

If `krond` is down during a chosen execution time:

On restart:

1. Compute period for current time.
2. Determine whether chosen time is in the past.
3. Apply deadline policy.

If `deadline` is zero:

* Missed periods are skipped.
* Only future periods are scheduled.

If `deadline` is non-zero:

* If now <= chosen + deadline:

  * Execute immediately.
* Otherwise:

  * Skip period.

No historical backlog is ever replayed beyond one bounded period.

---

## Output Handling

Child process STDOUT and STDERR are handled by:

* Inherit mode (default)
* Redirect to file
* Redirect to syslog
* Discard

If redirected to file:

* File path may include time formatting tokens.
* Files are opened before fork.
* Parent ensures file descriptors are closed.

Exit codes are logged.

---

## Signal Handling

`krond` handles:

* `SIGTERM`
* `SIGINT`
* `SIGHUP`

### SIGTERM / SIGINT

1. Stop accepting new triggers.
2. Allow active child processes to complete.
3. Optionally terminate active processes if configured.
4. Persist state.
5. Exit cleanly.

### SIGHUP

1. Reload configuration.
2. Validate new schedules.
3. Recompute queue.
4. Preserve active child processes.

Invalid new configuration does not replace old configuration.

---

## Locking Model

`krond` enforces single-daemon per system using:

* PID file with file lock.
* Or advisory lock on state directory.

If lock acquisition fails, daemon exits.

---

## Isolation and Security

* No command is executed through shell by default.
* No interpolation is performed unless explicitly configured.
* Environment variables must be explicitly defined to override inherited values.
* Privilege drop is required for user switching.

---

## Resource Limits

`krond` may configure:

* `RLIMIT_NOFILE`
* `RLIMIT_NPROC`
* Optional per-job limits if supported

System resource limits are not modified unless explicitly configured.

---

## Crash Recovery

If `krond` crashes:

* State files remain intact.
* On restart:

  * Active PID in state is verified.
  * If PID exists and matches expected command, it is considered active.
  * If PID does not exist, state is corrected.

No duplicate execution occurs for the same period.

---

## Determinism Guarantee

For each job and period:

* The chosen execution time is stable across daemon restarts.
* Seed derivation is stable.
* Fork timing may vary slightly due to scheduler latency, but chosen timestamp is invariant.

---

## Performance Model

`krond` must support:

* Thousands of schedule entries.
* Sub-second scheduling precision.
* O(log n) insertion/removal from scheduling queue.
* Minimal CPU usage when idle.

Memory usage scales linearly with number of entries.

---

## Failure Guarantees

Kron guarantees:

* At most one execution per period.
* No uncontrolled backfill.
* No duplicate execution due to restart.
* No hidden implicit retries.

Execution failures are surfaced through logs and exit codes only.

Kron does not retry failed commands unless explicitly configured in future versions.

---

## Summary

`krond` is:

* A single-process scheduler.
* Deterministic in decision-making.
* Fork/exec based.
* Policy-driven in concurrency.
* Bounded in catch-up behavior.
* Transparent in logging.
* Safe by default.
