# CRON-MIGRATION.md

## Purpose

This guide defines a practical migration path from system cron files to Kron daemon execution.

Target audience:

- operators migrating existing `/etc/crontab` and `/etc/cron.d/*` workloads
- teams that need deterministic scheduling behavior without a full rewrite on day one

---

## Migration Strategy

Use a two-stage migration:

1. **Compatibility stage**: run existing cron files through `krond --source cron`.
2. **Native stage**: convert selected jobs to Kron config format (`--source kron`) for richer controls.

This staged approach reduces operational risk and preserves rollback options.

---

## Stage 1: Compatibility Mode

### Supported Inputs

- `/etc/crontab` style entries: `m h dom mon dow user command...`
- `/etc/cron.d/*` style files
- environment assignment lines (`PATH=...`, `TZ=...`, `CRON_TZ=...`)
- common cron macros currently handled by parser (`@hourly`, `@daily`, `@weekly`, `@monthly`, `@yearly`)

### Unsupported or Deferred

- `@reboot`
- 6-field cron (seconds field)
- `run-parts` directory conventions
- `MAILTO` mail-delivery behavior
- non-root user/group switching (requires running `krond` as root for cross-account execution)

### Start Commands

Single file:

```bash
krond start --source cron --config /etc/crontab --state-dir /var/lib/krond
```

Directory:

```bash
krond start --source cron --config /etc/cron.d --state-dir /var/lib/krond
```

Dry-run one scheduler step:

```bash
krond start --source cron --config /etc/cron.d --state-dir /var/lib/krond --once
```

---

## Stage 2: Native Kron Conversion

After compatibility-stage stabilization, convert selected jobs to Kron-native entries.

### Field Mapping

| Cron Source | Kron Native |
|---|---|
| `m h dom mon dow user command` | `m h dom mon dow ... name=<name> command=<command>` |
| `TZ` / `CRON_TZ` | `@tz(<zone>)` |
| fixed schedule | same 5-field schedule |
| shell command text | `command=<...>` with `shell=true` when shell parsing is required |

### Native Enhancement Options

Use Kron modifiers where needed:

- `@win(...)` for deterministic execution windows
- `@dist(...)` for controlled spread (`uniform`, `skewEarly`, `skewLate`)
- `@seed(...)` for reproducible rotation behavior
- `@only(...)` / `@avoid(...)` for explicit execution constraints
- `@policy(...)` for deadline/concurrency semantics

---

## Validation Checklist

Before cutover:

- run parser/load checks in staging with `--once`
- verify all expected jobs are discovered from target files
- verify unsupported syntax is reported and triaged
- confirm state directory is writable and persistent

During cutover:

- keep previous cron service disabled but recoverable
- start `krond` with explicit `--source` and `--config`
- monitor first execution window for misses/skips

After cutover:

- verify no duplicate executions for same period
- verify command environment expectations (`PATH`, `TZ`, `CRON_TZ`)
- verify restart behavior preserves idempotency

---

## Rollback Plan

If production behavior diverges:

1. stop `krond`
2. re-enable previous cron service
3. retain `krond` state directory for postmortem
4. capture failing entries and classify as Tier 1/Tier 2/Tier 3 gaps

Rollback should not delete state files; preserve evidence for deterministic replay analysis.

---

## Recommended Order

1. Migrate stateless, low-risk jobs first.
2. Migrate business-critical jobs after one full observation cycle.
3. Convert high-value jobs to native Kron format for improved control and explainability.
