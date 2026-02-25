# SYNTAX.md

## Scope

This document defines the user-facing schedule syntax accepted by Kron. The same syntax is used by:

* Kubernetes CRDs (`KronJob.spec.schedule`, `KronJob.spec.window`, `KronJob.spec.distribution`, `KronJob.spec.constraints`)
* `krontab` entries for `krond`

The syntax is designed to be:

* Cron-compatible for the baseline schedule
* Explicit for probabilistic extensions
* Deterministic and fully parseable

---

## Grammar overview

A Kron schedule is composed of:

1. A baseline cron expression
2. Optional modifiers

Canonical form:

```
<cron> [@tz(<tz>)] [@win(<mode>,<duration>)] [@dist(<name>[,<k=v>...])] [@seed(<strategy>[,<k=v>...])] [@policy(<k=v>...)] [@avoid(<spec>)] [@only(<spec>)]
```

Order of modifiers is not significant.

Whitespace separates tokens. Parentheses do not contain unescaped whitespace.

---

## Cron expression

Kron accepts standard 5-field cron:

```
<minute> <hour> <day-of-month> <month> <day-of-week>
```

Ranges, lists, and steps are supported:

* `*`
* `n`
* `n-m`
* `n,m,k`
* `*/s`
* `n-m/s`

Aliases are supported where common:

* months: `JAN..DEC`
* weekdays: `SUN..SAT`

Day-of-week values:

* `0-6` with `0=SUN`
* `SUN..SAT`

Examples:

```
0 0 * * *           # daily at midnight
15 9 * * MON-FRI    # weekdays at 09:15
*/5 * * * *         # every 5 minutes
0 3 1 * *           # monthly on the 1st at 03:00
```

---

## Timezone modifier

```
@tz(<tz>)
```

`<tz>` is an IANA timezone name.

Examples:

```
0 10 * * * @tz(Europe/Paris)
0 18 * * * @tz(America/Los_Angeles)
```

If omitted, timezone is `UTC`.

---

## Window modifier

```
@win(<mode>,<duration>)
```

`<mode>`:

* `after`
* `around`

`<duration>` is a Go-style duration:

* `ns`, `us`, `ms`, `s`, `m`, `h`
* composite forms are allowed: `1h30m`

Semantics:

* `after`: window is `[nominal, nominal + duration]`
* `around`: window is `[nominal - duration/2, nominal + duration/2]`

Examples:

```
0 0 * * * @win(after,2h)
0 10 * * * @win(around,45m)
```

If omitted, window is `after,0s`.

---

## Distribution modifier

```
@dist(<name>[,<k=v>...])
```

`<name>`:

* `uniform`
* `normal`
* `skewEarly`
* `skewLate`
* `exponential`

Parameters are `k=v` pairs. Unknown keys are invalid.

MVP runtime status:

* `krontab explain` and `krontab next` execute: `uniform`, `skewEarly`, `skewLate`
* `normal` and `exponential` are syntax-valid and lint-validated, but runtime execution is not part of current MVP

### uniform

```
@dist(uniform)
```

No parameters.

### normal

```
@dist(normal[,mu=<anchor>][,sigma=<duration>])
```

MVP runtime note: syntax-valid and lint-validated, but not executed by `krontab explain`/`krontab next`.

* `mu` sets the mean anchor point.
* `sigma` sets standard deviation.

`mu` anchors:

* `nominal`
* `start`
* `mid`
* `end`

Defaults:

* `mu=nominal` for `around`
* `mu=mid` for `after`
* `sigma=window/6`

Examples:

```
0 10 * * * @win(around,2h) @dist(normal,sigma=20m)
0 10 * * * @win(after,2h)  @dist(normal,mu=start,sigma=15m)
```

### skewEarly / skewLate

```
@dist(skewEarly[,shape=<float>])
@dist(skewLate[,shape=<float>])
```

* `shape` controls skew intensity.
* `shape` is a positive float.
* default `shape=2.0`

Examples:

```
0 10 * * * @win(after,2h) @dist(skewEarly,shape=3)
0 10 * * * @win(after,2h) @dist(skewLate,shape=1.5)
```

### exponential

```
@dist(exponential[,lambda=<float>][,dir=<dir>])
```

MVP runtime note: syntax-valid and lint-validated, but not executed by `krontab explain`/`krontab next`.

* `lambda` is a positive float controlling decay rate.
* `dir`:

  * `early` biases toward window start
  * `late` biases toward window end
* defaults: `lambda=1.0`, `dir=early`

Examples:

```
0 10 * * * @win(after,2h) @dist(exponential,dir=early,lambda=1.2)
0 10 * * * @win(after,2h) @dist(exponential,dir=late,lambda=0.8)
```

If omitted, distribution is `uniform`.

---

## Seed modifier

```
@seed(<strategy>[,<k=v>...])
```

`<strategy>`:

* `stable`
* `daily`
* `weekly`

Parameters:

* `salt=<string>`

`<string>` is a UTF-8 string without unescaped whitespace. Quoted strings are supported:

* `"..."` with `\"` escapes

Examples:

```
0 0 * * * @seed(stable,salt="team-a")
0 10 * * * @seed(daily)
0 10 * * MON @seed(weekly,salt=prod)
```

If omitted, seed strategy is `stable` with empty salt.

---

## Policy modifier

```
@policy(<k=v>[,<k=v>...])
```

Keys:

* `concurrency=allow|forbid|replace`
* `deadline=<duration>`
* `suspend=true|false`

Defaults:

* `concurrency=forbid`
* `deadline=0s` (skip missed periods)
* `suspend=false`

Examples:

```
0 0 * * * @policy(concurrency=forbid,deadline=10m)
0 10 * * * @policy(concurrency=replace)
```

---

## Constraints modifiers

Constraints restrict when a chosen time may be selected. Constraints apply after the window is computed and before a final time is accepted. If a candidate time violates constraints, a new candidate is drawn. If no valid time can be found, the period is unschedulable.

### Avoid modifier

```
@avoid(<spec>)
```

### Only modifier

```
@only(<spec>)
```

A constraint spec is a semicolon-separated list of clauses:

```
<clause> ; <clause> ; ...
```

Clause forms:

* `hours=<range>`
* `dow=<range>`
* `dom=<range>`
* `months=<range>`
* `between=<hh:mm>-<hh:mm>`
* `date=<yyyy-mm-dd>`
* `dates=<yyyy-mm-dd>..<yyyy-mm-dd>`

Ranges:

* numeric ranges with lists and hyphens, e.g. `1-5,7,9-12`
* weekdays: `SUN..SAT`
* months: `JAN..DEC`

Time values are interpreted in the schedule timezone.

Examples:

```
0 10 * * * @win(after,2h) @avoid(hours=0-6)
0 18 * * * @win(around,1h) @only(dow=MON-FRI;between=08:00-20:00)
0 19 * * * @avoid(date=2026-12-25)
0 19 * * * @avoid(dates=2026-12-24..2026-12-31)
```

---

## Validation rules

* Cron expression must be valid and resolvable.
* Timezone must be a valid IANA name.
* Window duration must be non-negative.
* For `around` mode, `duration/2` is computed with nanosecond precision.
* Distributions must be recognized; parameters must be valid and within allowed ranges.
* In current MVP runtime, only `uniform`, `skewEarly`, and `skewLate` are executable in `explain`/`next`.
* Seed strategy must be recognized.
* Policy keys must be recognized; values must be valid.
* Constraint specs must be parseable; unknown clause keys are invalid.
* `@only(...)` and `@avoid(...)` may both be present.
* If constraints eliminate the entire window, the period is unschedulable.

---

## Examples (MVP executable with `explain`/`next`)

### Load smoothing (spread backups)

```
0 0 * * * @tz(UTC) @win(after,3h) @dist(uniform) @seed(stable,salt=backup) @policy(concurrency=forbid,deadline=30m)
```

### Human-like messaging (late-biased)

```
0 10 * * * @tz(Europe/Paris) @win(around,90m) @dist(skewLate,shape=2.5) @seed(daily,salt=msgs) @only(dow=MON-FRI;between=08:00-20:00)
```

### Home automation (gentle unpredictability)

```
0 18 * * * @tz(America/Los_Angeles) @win(around,20m) @dist(skewLate,shape=1.4) @seed(daily,salt=lights)
```

### Avoid nights and weekends

```
0 * * * * @tz(UTC) @win(after,30m) @dist(skewEarly) @only(dow=MON-FRI;hours=8-18)
```

### Monthly with wide window, strict deadline

```
0 2 1 * * @tz(UTC) @win(after,8h) @dist(uniform) @policy(concurrency=forbid,deadline=15m)
```

## Lint-only Examples (planned runtime distributions)

These pass `krontab lint` but are not executable by MVP `krontab explain`/`krontab next`.

```
0 10 * * * @win(around,2h) @dist(normal,sigma=20m)
0 10 * * * @win(after,2h)  @dist(exponential,dir=early,lambda=1.2)
```
