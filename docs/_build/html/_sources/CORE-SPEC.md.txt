# CORE-SPEC.md

## Scope

This document defines the formal contract of `kron-core`.

`kron-core` is a pure, deterministic scheduling engine.
It contains no side effects, no I/O, no persistence, and no adapter-specific logic.

All adapters (`krond`, `kron-operator`, future integrations) must treat this specification as authoritative for scheduling behavior.

---

## Design Constraints

`kron-core` must be:

* Pure: no external state access.
* Deterministic: identical inputs produce identical outputs.
* Stateless: all required state is provided as input.
* Time-explicit: no internal clock access.
* Side-effect free.

---

## Versioning

`kron-core` exposes a semantic version.

A change that alters:

* window calculation
* seed derivation
* distribution math
* constraint evaluation
* determinism guarantees

requires a major version increment.

---

## Core Concepts

### Identity

A stable string identifying the job:

```
Identity string
```

Must be UTF-8 and non-empty.

---

### Period

A period is defined solely by:

```
NominalTime (UTC instant)
Timezone (IANA)
```

`kron-core` does not compute cron schedules unless explicitly provided a schedule resolver.

---

## Primary Interfaces

### DecisionRequest

The engine consumes a single request object:

```
DecisionRequest {
    Identity        string
    NominalTime     time.Time (UTC)
    Timezone        string
    WindowMode      enum {After, Around}
    WindowDuration  time.Duration
    Distribution    DistributionSpec
    SeedStrategy    SeedSpec
    Salt            string
    Constraints     ConstraintSpec
}
```

All times are interpreted in UTC except where constraints require timezone interpretation.

---

### DecisionResult

The engine produces:

```
DecisionResult {
    PeriodID        string
    NominalTime     time.Time (UTC)
    WindowStart     time.Time (UTC)
    WindowEnd       time.Time (UTC)
    ChosenTime      time.Time (UTC)
    SeedHash        string
    Distribution    DistributionSpec
    ConstraintMeta  ConstraintMeta
}
```

If unschedulable:

```
DecisionResult {
    ...
    ChosenTime      nil
    Unschedulable   true
    Reason          string
}
```

---

## Determinism Requirements

For identical `DecisionRequest` values:

* `DecisionResult` must be byte-for-byte identical in:

  * `ChosenTime`
  * `SeedHash`
  * `WindowStart`
  * `WindowEnd`

Determinism must hold across:

* Process restarts
* Different machines
* Different architectures
* Supported Go versions within compatibility window

Floating point calculations must not introduce nondeterministic rounding across architectures.

If floating math is used internally, results must be quantized deterministically before producing `ChosenTime`.

---

## PeriodID

```
PeriodID = NominalTime.UTC().Format(RFC3339)
```

Must not depend on timezone formatting differences.

---

## Window Computation

Given:

```
N = NominalTime
D = WindowDuration
```

If `WindowMode=After`:

```
WindowStart = N
WindowEnd   = N + D
```

If `WindowMode=Around`:

```
Half = D / 2
WindowStart = N - Half
WindowEnd   = N + Half
```

If `D=0`:

```
WindowStart = N
WindowEnd   = N
ChosenTime  = N
```

No distribution sampling occurs when `WindowStart == WindowEnd`.

---

## Seed Derivation

Seed input string is constructed as:

```
SeedInput =
    Identity + "\n" +
    PeriodKey + "\n" +
    Salt
```

`PeriodKey` depends on seed strategy.

### Seed Strategies

#### Stable

```
PeriodKey = PeriodID
```

#### Daily

```
PeriodKey = YYYY-MM-DD in Timezone corresponding to NominalTime
```

#### Weekly

```
PeriodKey = ISOWeek(YYYY-Www) in Timezone corresponding to NominalTime
```

---

## Seed Hash

A cryptographic hash function must be used.

The hash function must:

* Be fixed for a major version.
* Produce identical results across platforms.
* Be documented in implementation.

Output encoding:

```
SeedHash = lowercase hexadecimal string
```

The full hash output must be used for PRNG seeding.

---

## PRNG Requirements

The pseudorandom generator:

* Must be deterministic for a given SeedHash.
* Must produce reproducible sequences.
* Must not depend on runtime-global randomness.

Initialization:

```
PRNG(seed = SeedHash bytes)
```

Sequence generation order must be stable and documented.

---

## Distribution Specification

```
DistributionSpec {
    Name string
    Params map[string]string
}
```

Parameter parsing must be deterministic.

Unknown parameters must cause error.

---

## Distribution Contract

Given:

```
U = PRNG.NextFloat64()
```

`U` must satisfy:

```
0 ≤ U < 1
```

Distributions must transform `U` into an offset within:

```
[0, WindowDuration]
```

or symmetrically around nominal in `Around` mode.

The final offset must be clamped to window bounds.

---

## Constraint Specification

```
ConstraintSpec {
    OnlyClauses  []Clause
    AvoidClauses []Clause
}
```

Constraint evaluation must:

* Interpret time in specified Timezone.
* Be pure.
* Not modify state.

---

## Candidate Selection Algorithm

1. Initialize PRNG with SeedHash.
2. For attempt in `0..MaxAttempts-1`:

   * Generate `U`.
   * Compute candidate offset via distribution.
   * Compute candidate time.
   * If candidate satisfies constraints:

     * Return candidate.
3. If no candidate accepted:

   * Return Unschedulable.

`MaxAttempts` must be fixed and documented.

`MaxAttempts` must be deterministic and independent of runtime conditions.

---

## Constraint Evaluation Rules

Given candidate time `T`:

* Convert `T` to schedule timezone.
* Evaluate `OnlyClauses`:

  * If present, `T` must satisfy at least one clause set.
* Evaluate `AvoidClauses`:

  * If any clause matches, candidate rejected.

Clause types and evaluation logic are defined in SYNTAX.md.

Constraint evaluation must not modify `T`.

---

## Error Handling

The engine must return explicit errors for:

* Invalid timezone
* Invalid distribution name
* Invalid distribution parameters
* Invalid seed strategy
* Invalid constraint definitions
* Negative window duration

Errors must not panic.

---

## Timezone Handling

Timezone must be resolved via IANA database.

If timezone is invalid:

* Return error.

All conversions must be deterministic given identical timezone database.

Implementations must document supported timezone database source.

---

## Precision Rules

* Internal calculations may use nanoseconds.
* Final `ChosenTime` must be truncated or rounded in a deterministic manner.
* Rounding policy must be stable and documented.

ChosenTime must never fall outside window bounds due to rounding.

---

## Unschedulable Semantics

If no valid candidate found:

```
DecisionResult.Unschedulable = true
DecisionResult.ChosenTime = zero value
```

`SeedHash`, `WindowStart`, `WindowEnd`, and `PeriodID` must still be populated.

---

## Stability Guarantees

For a fixed major version:

* Seed derivation format must not change.
* Hash algorithm must not change.
* PRNG algorithm must not change.
* Distribution math must not change.
* Window computation must not change.

Any change affecting output for identical inputs requires major version increment.

---

## Performance Requirements

`Decision()` must:

* Execute in bounded time.
* Be O(1) relative to window duration.
* Not allocate unbounded memory.
* Not depend on external services.

---

## Prohibited Behaviors

`kron-core` must not:

* Access system clock.
* Perform I/O.
* Access filesystem.
* Access network.
* Read environment variables.
* Use global mutable state.
* Use non-deterministic randomness.

---

## Conformance

A conforming implementation must:

* Pass all golden test vectors defined in TEST-VECTORS.md.
* Produce identical outputs across supported architectures.
* Preserve determinism guarantees.

Adapters must not alter or reinterpret `DecisionResult` fields.

`kron-core` is the authoritative source of scheduling truth.
