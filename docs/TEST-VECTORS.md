# TEST-VECTORS.md

## Canonical algorithms for vectors

These test vectors are defined against a fixed, fully specified core algorithm set.

## Machine-readable vectors

The executable golden vectors used by `kron-core` tests live in:

* `core/testdata/vectors/v1.json`
* `core/testdata/vectors/v2.json`
* `core/testdata/vectors/v3.json`
* `core/testdata/vectors/v4.json`
* `core/testdata/vectors/v5.json`
* `core/testdata/vectors/v6.json`
* `core/testdata/vectors/v7.json`

Current implementation coverage in that file focuses on implemented MVP behavior:

* `uniform`, `skewEarly`, and `skewLate` distributions (including skew shape parameter coverage)
* `after`, `before`, and `around`/`center` window behavior
* zero-duration window behavior
* seed strategies (`stable`, `daily`, `weekly`)
* constraint handling (`hours`, `dow`, `between`, `dom`, `months`, `date`/`dates`) and unschedulable outcomes
* edge cases: timezone period-key boundaries and odd `around` durations

### Seed hash

* `HASH = SHA-256`
* `SeedInput = Identity + "\n" + PeriodKey + "\n" + Salt`
* `SeedHash = hex(lowercase(SHA-256(SeedInput)))`

### PRNG

* `PRNG = SplitMix64`
* `seed_u64 = uint64_be(SHA-256(SeedInput)[0:8])`
* SplitMix64 step:

```
x = x + 0x9E3779B97F4A7C15
z = x
z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
z = (z ^ (z >> 27)) * 0x94D049BB133111EB
z = z ^ (z >> 31)
return z
```

* `NextFloat64()`:

```
u = next_uint64()
f = (u >> 11) / 2^53
```

So `0 ≤ f < 1`.

### Window mapping

Let `W = window_end - window_start` in whole seconds, where `W ≥ 0`.

* If `W == 0`: `chosen_time = window_start`.
* Else, for a unit value `x` in `[0,1)`:

```
offset_seconds = floor(x * (W + 1))
chosen_time = window_start + offset_seconds seconds
```

This mapping permits selecting `window_end` when `offset_seconds == W`.

### Distributions

Let `u = NextFloat64()`.

* `uniform`:

  * `x = u`
* `skewEarly(shape=s)` where `s > 0`:

  * `x = u^s`
* `skewLate(shape=s)` where `s > 0`:

  * `x = 1 - (1 - u)^s`

Constraints are evaluated after mapping to a candidate time, using deterministic resampling with the same PRNG stream.

### Constraint sampling budget

* `MaxAttempts = 64`
* Attempts consume one `NextFloat64()` per attempt.

If no candidate satisfies constraints after `MaxAttempts`, the period is `unschedulable`.

---

## Notation

* Times are RFC3339 UTC.
* `PeriodID = NominalTime.UTC().Format(RFC3339)`.
* For `SeedStrategy=stable`, `PeriodKey = PeriodID`.
* For `SeedStrategy=daily`, `PeriodKey = YYYY-MM-DD in Timezone corresponding to NominalTime`.
* For `SeedStrategy=weekly`, `PeriodKey = ISO week YYYY-Www in Timezone corresponding to NominalTime`.

---

## Core decision vectors

### V1 — uniform, after-window, stable seed

**Request**

* `Identity`: `prod/db-backup`
* `NominalTime`: `2026-03-01T00:00:00Z`
* `Timezone`: `UTC`
* `WindowMode`: `after`
* `WindowDuration`: `3h` (`W=10800`)
* `Distribution`: `uniform`
* `SeedStrategy`: `stable`
* `Salt`: `backup`
* `Constraints`: none

**Expected**

* `PeriodID`: `2026-03-01T00:00:00Z`
* `WindowStart`: `2026-03-01T00:00:00Z`
* `WindowEnd`: `2026-03-01T03:00:00Z`
* `SeedHash`: `9c85657760a63b4d925af6088cceb2bb4448380b2e6856b203915a0a51ab5101`
* `First u`: `0.8462881248863515`
* `offset_seconds`: `9140`
* `ChosenTime`: `2026-03-01T02:32:20Z`
* `Unschedulable`: `false`

---

### V2 — skewLate, around-window, stable seed

**Request**

* `Identity`: `msgs/paris`
* `NominalTime`: `2026-03-02T09:00:00Z`
* `Timezone`: `Europe/Paris`
* `WindowMode`: `around`
* `WindowDuration`: `90m` (`W=5400`)
* `Distribution`: `skewLate(shape=2.5)`
* `SeedStrategy`: `stable`
* `Salt`: `msgs`
* `Constraints`: none

**Expected**

* `PeriodID`: `2026-03-02T09:00:00Z`
* `WindowStart`: `2026-03-02T08:15:00Z`
* `WindowEnd`: `2026-03-02T09:45:00Z`
* `SeedHash`: `8b95acf566414238f55eb4541a1bc726b80d02fe86a0cd2ad52988a74860b2f5`
* `First u`: `0.4757178150383121`
* `x = 1 - (1-u)^2.5`: `0.800972654023368`
* `offset_seconds`: `4326`
* `ChosenTime`: `2026-03-02T09:27:06Z`
* `Unschedulable`: `false`

---

### V3 — daily seed strategy

**Request**

* `Identity`: `daily/test`
* `NominalTime`: `2026-03-01T00:00:00Z`
* `Timezone`: `UTC`
* `WindowMode`: `after`
* `WindowDuration`: `1h` (`W=3600`)
* `Distribution`: `uniform`
* `SeedStrategy`: `daily`
* `Salt`: empty
* `Constraints`: none

**Expected**

* `PeriodID`: `2026-03-01T00:00:00Z`
* `PeriodKey`: `2026-03-01`
* `WindowStart`: `2026-03-01T00:00:00Z`
* `WindowEnd`: `2026-03-01T01:00:00Z`
* `SeedHash`: `3a1cbafc74e05e46dc6a4eff53a9d71da286eda9585a70c5c19bd43c52763161`
* `First u`: `0.3528233669308106`
* `offset_seconds`: `1270`
* `ChosenTime`: `2026-03-01T00:21:10Z`
* `Unschedulable`: `false`

---

### V4 — zero window duration (no sampling)

**Request**

* `Identity`: `exact/nojitter`
* `NominalTime`: `2026-01-01T00:00:00Z`
* `Timezone`: `UTC`
* `WindowMode`: `after`
* `WindowDuration`: `0s` (`W=0`)
* `Distribution`: `uniform`
* `SeedStrategy`: `stable`
* `Salt`: empty
* `Constraints`: none

**Expected**

* `PeriodID`: `2026-01-01T00:00:00Z`
* `WindowStart`: `2026-01-01T00:00:00Z`
* `WindowEnd`: `2026-01-01T00:00:00Z`
* `SeedHash`: `8b0e1ef5c9c9886e07842b8f00c04697f5257c68188a33de362a414012b4eb84`
* `ChosenTime`: `2026-01-01T00:00:00Z`
* `Unschedulable`: `false`

---

### V5 — constraints cause unschedulable (sampling budget exceeded)

**Constraint model**

* `Only`: `between=18:58-19:00` in `UTC` (inclusive bounds)
* Equivalent allowed offset range (relative to window start): `3500..3600` seconds inclusive
* `MaxAttempts = 64`

**Request**

* `Identity`: `home/lights`
* `NominalTime`: `2026-03-01T18:00:00Z`
* `Timezone`: `UTC`
* `WindowMode`: `after`
* `WindowDuration`: `1h` (`W=3600`)
* `Distribution`: `uniform`
* `SeedStrategy`: `stable`
* `Salt`: `lights`
* `Constraints`: `Only(between=18:58-19:00)`

**Expected**

* `PeriodID`: `2026-03-01T18:00:00Z`
* `WindowStart`: `2026-03-01T18:00:00Z`
* `WindowEnd`: `2026-03-01T19:00:00Z`
* `SeedHash`: `d61bedd4e238549ebdb9d45993ea58904aa8042c4691967c720b0f2006416345`
* `ChosenTime`: empty
* `Unschedulable`: `true`
* `Reason`: `no candidate accepted within MaxAttempts`

---

### V6 — calendar constraints (`dom`, `months`, `date`/`dates`) in zero-window cases

`v6.json` adds deterministic zero-window vectors to validate day/month/date-range constraint behavior.

Coverage in this vector set:

* schedulable candidate with combined `Only(dom, months, dates)`
* unschedulable candidate with `Avoid(dom)`
* unschedulable candidate with `Avoid(months)`
* unschedulable candidate with `Avoid(dates range)`

---

### V7 — skew-shape parameter determinism

`v7.json` adds deterministic vectors for `skewLate` with explicit `shape` values.

Coverage in this vector set:

* explicit `shape=1.5`
* explicit `shape=2.5`
* stable output differences from shape changes under fixed period/window/seed strategy

---

## Adapter outcome vectors

These vectors combine a core decision with policy evaluation at a specific `now`.

### A1 — missed with deadline=0s

**Decision**

Use V1 decision.

* `ChosenTime`: `2026-03-01T02:32:20Z`

**Policy**

* `deadline = 0s`
* `concurrency = forbid`
* `suspend = false`

**Input**

* `now = 2026-03-01T02:32:21Z`
* `active_execution = false`
* `state: period not handled yet`

**Expected outcome**

* Terminal outcome: `missed`
* No execution started
* Period marked handled as `missed`

---

### A2 — executed within deadline

**Decision**

Use V1 decision.

* `ChosenTime`: `2026-03-01T02:32:20Z`

**Policy**

* `deadline = 10m`
* `concurrency = forbid`
* `suspend = false`

**Input**

* `now = 2026-03-01T02:40:00Z`
* `active_execution = false`
* `state: period not handled yet`

**Expected outcome**

* Terminal outcome: `executed`
* Execution started once
* Period marked handled as `executed`

---

### A3 — skipped due to forbid concurrency

**Decision**

Use V3 decision.

* `ChosenTime`: `2026-03-01T00:21:10Z`

**Policy**

* `deadline = 30m`
* `concurrency = forbid`
* `suspend = false`

**Input**

* `now = 2026-03-01T00:25:00Z`
* `active_execution = true`

**Expected outcome**

* Terminal outcome: `skipped`
* No new execution started
* Period marked handled as `skipped`

---

### A4 — idempotency prevents duplicate execution

**Decision**

Use V1 decision.

**Policy**

* `deadline = 10m`
* `concurrency = allow`
* `suspend = false`

**Input**

* `now = 2026-03-01T02:35:00Z`
* `active_execution = false`
* `state: LastHandledPeriodID == 2026-03-01T00:00:00Z with outcome=executed`

**Expected outcome**

* No execution started
* Period remains handled
* Adapter reports `job_exists` / `already handled` behavior
