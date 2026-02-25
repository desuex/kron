# COMPAT.md

## Scope

This document defines compatibility and non-compatibility between Kron and existing scheduling systems.

It covers:

* Cron compatibility
* anacron compatibility
* systemd timer comparison
* Kubernetes CronJob comparison
* Migration semantics
* Explicit differences

Kron aims for predictable behavior, not bug-for-bug compatibility.

---

## Cron Drop-in Profile (Next Stage)

Kron defines "drop-in replacement" as high compatibility for common cron workloads, not full implementation-specific parity.

The staged compatibility matrix and acceptance criteria for the next stage are defined in:

* [`docs/CRON-DROPIN.md`](CRON-DROPIN.md)

This profile is the contract for `krond` host-daemon development.

---

## Cron Expression Compatibility

Kron supports standard 5-field cron syntax:

```
<minute> <hour> <day-of-month> <month> <day-of-week>
```

Supported features:

* `*`
* numeric values
* ranges (`n-m`)
* lists (`a,b,c`)
* steps (`*/s`, `n-m/s`)
* month names (`JAN..DEC`)
* weekday names (`SUN..SAT`)

Unsupported:

* seconds field (6-field cron)
* `@reboot`
* `@yearly`, `@monthly`, etc. shortcuts
* nonstandard extensions from specific cron implementations

---

## Day-of-Month and Day-of-Week Semantics

Kron follows standard cron OR semantics:

A schedule matches when:

```
month matches AND
(
  day-of-month matches OR
  day-of-week matches
)
```

This behavior matches Vixie cron.

This is stable and guaranteed.

---

## Timezone Semantics

Unlike traditional cron:

* Kron supports per-job timezone specification.
* Cron typically uses system timezone only.

Kron interprets:

* Schedule in specified timezone.
* Constraints in specified timezone.
* Internally stores UTC instants.

This may produce different behavior than system cron in multi-timezone environments.

---

## DST Handling

Kron defines deterministic DST behavior.

When clocks move forward:

* Nominal times that do not exist are skipped.
* No synthetic time is created.

When clocks move backward:

* Duplicate wall-clock times are resolved to distinct UTC instants.
* Each nominal UTC instant corresponds to one period.

Kron does not attempt to “run twice” for repeated wall-clock times unless the cron schedule resolves to two distinct UTC instants.

Behavior may differ from certain cron implementations.

---

## anacron Comparison

anacron:

* Runs jobs that were missed while system was down.
* Guarantees eventual daily execution.

Kron:

* Does not guarantee eventual execution by default.
* Uses explicit `deadline` policy.
* Never replays unlimited backlog.

To approximate anacron behavior:

* Use daily schedule.
* Use `@seed(daily)`.
* Set appropriate `deadline`.

Kron prioritizes bounded execution over guaranteed replay.

---

## systemd Timers Comparison

systemd timers support:

* OnCalendar scheduling
* RandomizedDelaySec (uniform jitter)
* Persistent=true (catch-up)

Differences:

* Kron supports biased distributions.
* Kron provides deterministic seeding.
* Kron exposes explainable decision metadata.
* Kron enforces at most one execution per period.

systemd randomization is typically uniform and non-deterministic across restarts.

Kron is deterministic across restarts.

---

## Kubernetes CronJob Comparison

Kubernetes CronJob:

* Exact schedule execution.
* Optional `startingDeadlineSeconds`.
* Concurrency policies.
* No distribution control.
* No deterministic jitter.

Kron:

* Adds windowed scheduling.
* Adds biased distributions.
* Adds deterministic seed strategies.
* Adds constraint system.
* Adds explainable scheduling.

Kron can replicate CronJob behavior with:

```
@win(after,0s)
@dist(uniform)
@seed(stable)
```

Behavior then matches exact schedule semantics.

---

## Behavior Differences from Cron

1. Deterministic randomness:

   * Traditional cron has none.
   * Kron introduces window and distribution.

2. No implicit shell:

   * Cron uses `/bin/sh` by default.
   * Kron executes directly unless explicitly configured.

3. No mail on failure:

   * Cron may mail output.
   * Kron logs explicitly.

4. No automatic retries:

   * Cron does not retry.
   * Kron does not retry.

5. Bounded catch-up:

   * Cron may execute missed jobs on restart.
   * Kron does not unless within deadline.

---

## Backward Compatibility Rules

Within a major version:

* Cron parsing semantics must not change.
* Seed derivation must not change.
* Distribution math must not change.
* Window semantics must not change.
* Constraint semantics must not change.

New distributions may be added.

New constraint types may be added.

Defaults must remain stable.

---

## Forward Compatibility

If new syntax features are introduced:

* Older versions must reject unknown modifiers.
* Unknown modifiers must not be silently ignored.

Config written for newer versions is not guaranteed to work on older versions.

---

## Migration from Cron

To migrate from cron:

1. Copy cron expression.
2. Add `@win(after,0s)` if explicitness desired.
3. Ensure command does not rely on shell features.
4. Configure logging as desired.

Cron jobs using shell pipelines must:

* Be wrapped explicitly in shell invocation.
* Or rewritten as script files.

---

## Migration from systemd Timers

Mapping:

* `OnCalendar` → cron expression
* `RandomizedDelaySec` → `@win(after,<duration>) @dist(uniform)`
* `Persistent=true` → set `@policy(deadline=<duration>)`

systemd-specific features such as `OnBootSec` have no direct equivalent.

---

## Migration from Kubernetes CronJob

Mapping:

* `spec.schedule` → `schedule`
* `startingDeadlineSeconds` → `@policy(deadline=<duration>)`
* `concurrencyPolicy` → `@policy(concurrency=...)`
* `suspend` → `@policy(suspend=true)`

To maintain identical behavior:

* Set window duration to `0s`.

---

## Precision Differences

Cron typically has minute-level precision.

Kron:

* Accepts minute-level cron.
* May choose second-level offsets within window.
* Execution trigger precision may exceed minute resolution.

This is intentional.

---

## Reproducibility Guarantee

Kron guarantees:

* Same config + same version + same inputs = same chosen time.

Traditional cron makes no such guarantee when jitter or randomness involved.

---

## Unsupported Cron Features

Kron does not support:

* `@reboot`
* Environment variable declarations in schedule file
* Special macros like `@hourly`
* Year field

If needed, adapters may translate such macros into canonical form before passing to core.

---

## Interoperability Expectations

Kron does not attempt to:

* Replace system cron transparently.
* Interpret crontab files without explicit conversion.
* Emulate platform-specific cron quirks.

Kron provides a well-defined, deterministic scheduling model.

---

## Compatibility Invariants

Kron guarantees:

1. Standard cron expressions behave predictably.
2. OR semantics for DOM/DOW.
3. Per-job timezone support.
4. Deterministic decision for same input.
5. Window and distribution do not alter nominal period identity.
6. Backward compatibility within major version.

Behavioral changes require major version increment.

---

## Summary

Kron is:

* Cron-compatible at baseline.
* Deterministic and distribution-aware.
* Not bug-for-bug compatible with any specific cron implementation.
* Explicit in semantics.
* Stable within major versions.

Compatibility is intentional, not accidental.
