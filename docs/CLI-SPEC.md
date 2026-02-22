# CLI-SPEC.md

## Scope

This document defines the command-line interface (CLI) specification for:

* `krontab` (configuration manager)
* `krond` (daemon control interface)
* `kronctl` (optional Kubernetes helper)

This specification defines:

* Commands
* Flags
* Exit codes
* Output formats
* Determinism guarantees
* Compatibility rules

All CLI behavior must be stable within a major version.

---

# krontab

`krontab` manages local Kron configuration files for `krond`.

## Configuration Location

Default:

```
/etc/krond.d/
```

Per-user mode (optional):

```
~/.config/krond/
```

Each file may contain one or more job entries.

---

## Command: `krontab lint`

Validates configuration.

```
krontab lint [--file <path>] [--format text|json]
```

Behavior:

* Parses configuration.
* Validates syntax.
* Validates distributions.
* Validates constraints.
* Validates durations.
* Does not require daemon running.

Exit codes:

* `0` valid
* `1` invalid
* `2` system error

Output (text):

```
OK: /etc/krond.d/backups.kron
```

Output (json):

```
{
  "file": "...",
  "valid": true,
  "errors": []
}
```

---

## Command: `krontab explain`

Explains decision for a specific job and period.

```
krontab explain <job> --at <RFC3339> [--format text|json]
```

Behavior:

* Loads job definition.
* Resolves nominal period for `--at`.
* Calls `kron-core`.
* Outputs full decision explanation.
* Does not execute job.

Exit codes:

* `0` success
* `1` job not found
* `2` invalid input

Text output example:

```
Identity: prod/db-backup
PeriodID: 2026-03-01T00:00:00Z
Window: [2026-03-01T00:00:00Z, 2026-03-01T03:00:00Z]
Distribution: uniform
SeedHash: 9c85657760a63b4d925af6088cceb2bb4448380b2e6856b203915a0a51ab5101
ChosenTime: 2026-03-01T02:32:20Z
```

JSON output must include full DecisionResult fields.

---

## Command: `krontab next`

Shows next N scheduled decisions.

```
krontab next <job> [--count N] [--format text|json]
```

Default `N=1`.

Behavior:

* Iteratively compute next periods using `kron-core`.
* Do not mutate state.
* Deterministic output.

Exit codes:

* `0` success
* `1` job not found

---

## Command: `krontab add`

Adds or updates job definition.

```
krontab add --file <path>
```

Behavior:

* Validates file.
* Writes to config directory.
* Does not restart daemon automatically.

Exit codes:

* `0` success
* `1` validation error
* `2` write error

---

## Command: `krontab remove`

Removes job.

```
krontab remove <job>
```

Behavior:

* Removes configuration entry.
* Does not delete state automatically.

Exit codes:

* `0` success
* `1` job not found

---

# krond

`krond` is the daemon.

---

## Command: `krond start`

```
krond start [--config <dir>] [--foreground] [--log-format text|json]
```

Behavior:

* Starts daemon.
* Acquires lock.
* Loads configuration.
* Enters scheduler loop.

Exit codes:

* `0` clean exit
* `1` configuration error
* `2` state error
* `3` lock error
* `4` fatal persistence error

---

## Command: `krond reload`

```
krond reload
```

Behavior:

* Sends SIGHUP to running daemon.
* Daemon reloads config.
* Active executions unaffected.

Exit codes:

* `0` success
* `1` daemon not running

---

## Command: `krond status`

```
krond status [--format text|json]
```

Outputs:

* Running/not running
* PID
* Number of jobs
* Next scheduled execution time (earliest)

Exit codes:

* `0` running
* `1` not running

---

## Command: `krond stop`

```
krond stop [--graceful]
```

Behavior:

* Sends SIGTERM.
* If `--graceful`, waits for active jobs to complete.

Exit codes:

* `0` success
* `1` daemon not running

---

# kronctl (Kubernetes helper)

Optional CLI for Kubernetes users.

---

## Command: `kronctl explain`

```
kronctl explain <namespace>/<kronjob> [--at <RFC3339>] [--format text|json]
```

Behavior:

* Fetch KronJob.
* Compute decision using kron-core.
* Does not create Job.
* Does not mutate cluster.

---

## Command: `kronctl next`

```
kronctl next <namespace>/<kronjob> [--count N]
```

Displays next N decisions.

---

## Command: `kronctl validate`

```
kronctl validate -f <file>
```

Validates YAML manifests before applying.

---

# Output Formats

All CLI tools must support:

* `text`
* `json`

Text:

* Human readable.
* Stable field labels.
* Deterministic ordering.

JSON:

* Single JSON object per command.
* Stable field names.
* No extraneous fields.
* Deterministic key order not required.

---

# Exit Codes

Global conventions:

* `0` success
* `1` user error (invalid input, not found)
* `2` validation/config error
* `3` state error
* `4` runtime/fatal error

Exit codes must not change within major version.

---

# Determinism Requirements

For identical:

* Config
* Version
* Inputs

Commands:

* `explain`
* `next`

Must produce identical outputs.

Output must not depend on:

* Current system time (unless explicitly provided)
* Environment variables
* Host locale

---

# Logging Interaction

CLI tools:

* Must not emit daemon logs.
* Must print only command output.
* Errors must go to stderr.
* Structured JSON must go to stdout only.

---

# Backward Compatibility

Within major version:

* Command names stable.
* Flag names stable.
* Output schema stable.
* Exit codes stable.

New commands may be added.

Breaking CLI changes require major version increment.

---

# Security Rules

CLI must:

* Validate file permissions when appropriate.
* Refuse to operate on world-writable config.
* Not expose sensitive environment variables.
* Not execute jobs during explain or validation.

---

# Non-Goals

CLI does not:

* Replace cron CLI exactly.
* Emulate crontab interactive editor.
* Automatically restart daemon on config change.
* Provide workflow orchestration.

CLI is a deterministic interface to Kron scheduling semantics.
