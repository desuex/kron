# LOGS.md

## Scope

This document defines the human-readable log format for Kron.

The log format must:

* Be readable without external tooling.
* Be deterministic and reproducible.
* Allow operators to answer “why did this run at this time?”
* Be parseable by log aggregation systems.
* Remain stable across minor releases.

Logs are structured, single-line, key-value entries.

---

## Log Principles

1. One event per line.
2. No multiline entries.
3. No hidden state.
4. All scheduling decisions are fully explainable from log content.
5. Stable field names.
6. Timestamps are always RFC3339 with nanosecond precision in UTC.

---

## Log Format

Each line has:

```
<timestamp> level=<level> component=<component> event=<event> <fields...>
```

Example:

```
2026-03-01T09:58:12.483920381Z level=INFO component=scheduler event=decision namespace=prod name=db-backup period=2026-03-01T00:00:00Z nominal=2026-03-01T00:00:00Z window_start=2026-03-01T00:00:00Z window_end=2026-03-01T03:00:00Z distribution=uniform seed_hash=4f8c2a13 chosen=2026-03-01T01:42:18Z
```

Fields are:

* space-separated
* key=value
* values containing spaces are quoted with double quotes
* quotes escaped with `\"`

---

## Levels

Levels:

* `DEBUG`
* `INFO`
* `WARN`
* `ERROR`

Default production level is `INFO`.

---

## Components

Component values:

* `scheduler`
* `controller`
* `executor`
* `constraints`
* `seed`
* `policy`
* `reconciler`

---

## Core Events

### decision

Emitted when a scheduling decision is computed.

Fields:

* `namespace`
* `name`
* `period`
* `nominal`
* `window_mode`
* `window_start`
* `window_end`
* `distribution`
* `dist_params`
* `seed_strategy`
* `seed_hash`
* `salt`
* `chosen`
* `timezone`

Example:

```
2026-03-01T00:00:00.001234567Z level=INFO component=scheduler event=decision namespace=prod name=db-backup period=2026-03-01T00:00:00Z nominal=2026-03-01T00:00:00Z window_mode=after window_start=2026-03-01T00:00:00Z window_end=2026-03-01T03:00:00Z distribution=uniform dist_params="" seed_strategy=stable seed_hash=4f8c2a13 salt=backup chosen=2026-03-01T01:42:18Z timezone=UTC
```

---

### reschedule

Emitted when a decision changes due to spec update.

Fields:

* `namespace`
* `name`
* `period`
* `previous_chosen`
* `new_chosen`
* `reason`

---

### trigger

Emitted when chosen time is reached and execution is attempted.

Fields:

* `namespace`
* `name`
* `period`
* `chosen`
* `now`

---

### job_created

Emitted when a Job is successfully created.

Fields:

* `namespace`
* `name`
* `period`
* `job`
* `uid`
* `chosen`

---

### job_exists

Emitted when a Job already exists for the period.

Fields:

* `namespace`
* `name`
* `period`
* `job`
* `uid`

---

### skipped_concurrency

Emitted when execution is skipped due to concurrency policy.

Fields:

* `namespace`
* `name`
* `period`
* `policy`
* `active_job`

---

### replaced_job

Emitted when an active Job is replaced.

Fields:

* `namespace`
* `name`
* `period`
* `old_job`
* `new_job`

---

### missed_deadline

Emitted when execution is skipped because deadline was exceeded.

Fields:

* `namespace`
* `name`
* `period`
* `chosen`
* `deadline`
* `now`

---

### constraint_reject

Emitted when a candidate time is rejected due to constraints.

Fields:

* `namespace`
* `name`
* `period`
* `candidate`
* `reason`

Multiple `constraint_reject` events may be emitted for a single decision when sampling multiple candidates.

---

### constraint_unsatisfiable

Emitted when no valid time can be found in the window.

Fields:

* `namespace`
* `name`
* `period`
* `window_start`
* `window_end`
* `reason`

---

### error

Emitted on operational errors.

Fields:

* `namespace`
* `name`
* `period` (if applicable)
* `operation`
* `error`

---

## Seed Transparency

When computing the seed, a `DEBUG` log is emitted:

Event:

```
event=seed_computed
```

Fields:

* `namespace`
* `name`
* `period`
* `seed_strategy`
* `resource_uid`
* `period_key`
* `salt`
* `seed_hash`

Example:

```
2026-03-01T00:00:00.000112233Z level=DEBUG component=seed event=seed_computed namespace=prod name=db-backup period=2026-03-01T00:00:00Z seed_strategy=stable resource_uid=7f12a8 period_key=2026-03-01T00:00:00Z salt=backup seed_hash=4f8c2a13
```

---

## Distribution Transparency

When computing a candidate time:

Event:

```
event=sample
```

Fields:

* `namespace`
* `name`
* `period`
* `distribution`
* `raw_value`
* `mapped_offset_ns`
* `candidate`

Example:

```
2026-03-01T00:00:00.000223344Z level=DEBUG component=scheduler event=sample namespace=prod name=db-backup period=2026-03-01T00:00:00Z distribution=uniform raw_value=0.57382912 mapped_offset_ns=6138000000000 candidate=2026-03-01T01:42:18Z
```

---

## Human Explanation Mode

When verbose logging is enabled, Kron emits a structured explanation block as sequential INFO lines:

Events:

* `explain_start`
* `explain_step`
* `explain_result`
* `explain_end`

Example:

```
2026-03-01T00:00:00Z level=INFO component=scheduler event=explain_start namespace=prod name=db-backup period=2026-03-01T00:00:00Z
2026-03-01T00:00:00Z level=INFO component=scheduler event=explain_step detail="cron resolved nominal time"
2026-03-01T00:00:00Z level=INFO component=scheduler event=explain_step detail="window after 3h => [00:00,03:00]"
2026-03-01T00:00:00Z level=INFO component=scheduler event=explain_step detail="uniform distribution selected 0.5738"
2026-03-01T00:00:00Z level=INFO component=scheduler event=explain_result chosen=2026-03-01T01:42:18Z
2026-03-01T00:00:00Z level=INFO component=scheduler event=explain_end namespace=prod name=db-backup period=2026-03-01T00:00:00Z
```

Explanation mode is optional and disabled by default.

---

## Ordering Guarantees

For a given `KronJob` and period:

1. `decision` must appear before `trigger`.
2. `trigger` must appear before `job_created` or `skipped_*`.
3. If constraints are evaluated, `constraint_reject` must appear before `decision` finalization.
4. `seed_computed` must appear before `sample`.

---

## Stability Guarantees

* Field names will not change in stable API versions.
* Additional fields may be appended.
* Field ordering within a line is not guaranteed.
* Events will not be renamed in stable API versions.

---

## Log Compatibility Modes

Kron supports:

* `text` (default, as defined here)
* `json` (same fields serialized as structured JSON)

In JSON mode, each line is a single JSON object with identical field keys.

Example:

```
{"timestamp":"2026-03-01T00:00:00Z","level":"INFO","component":"scheduler","event":"decision","namespace":"prod","name":"db-backup","period":"2026-03-01T00:00:00Z","nominal":"2026-03-01T00:00:00Z","window_mode":"after","window_start":"2026-03-01T00:00:00Z","window_end":"2026-03-01T03:00:00Z","distribution":"uniform","seed_strategy":"stable","seed_hash":"4f8c2a13","chosen":"2026-03-01T01:42:18Z"}
```
