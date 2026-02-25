# CRD-SPEC.md

## Scope

This document defines the Kubernetes Custom Resource Definition (CRD) contract for Kron.

It specifies:

* API group and versions
* Resource schema
* Defaulting rules
* Validation rules
* Status contract
* Immutability rules
* Upgrade guarantees

This specification applies to the Kubernetes adapter (`kron-operator`).

---

## API Group

```text
Group: kron.io
Version: v1alpha1
Kind: KronJob
Plural: kronjobs
Scope: Namespaced
```

Future versions must follow Kubernetes API versioning rules.

---

## Resource Structure

```text
apiVersion: kron.io/v1alpha1
kind: KronJob
metadata:
  name: string
  namespace: string
spec: KronJobSpec
status: KronJobStatus
```

---

## Spec

```text
KronJobSpec {
  schedule: string
  timezone: string (optional)
  window: WindowSpec
  distribution: DistributionSpec
  seed: SeedSpec
  constraints: ConstraintSpec (optional)
  policy: PolicySpec (optional)
  jobTemplate: batchv1.JobTemplateSpec
}
```

---

## schedule

* Required.
* Standard 5-field cron expression.
* Must pass validation at admission time.
* Interpreted in `timezone` if provided, otherwise `UTC`.

Invalid schedule causes resource rejection.

---

## timezone

* Optional.
* IANA timezone name.
* Default: `UTC`.

Invalid timezone causes resource rejection.

---

## window

```text
WindowSpec {
  mode: "after" | "around"
  duration: string (Go duration format)
}
```

Default:

```text
mode: "after"
duration: "0s"
```

Validation:

* duration must be ≥ 0.
* If `around`, duration may be zero but must not overflow when halved.
* Excessively large durations may be rejected by validation policy.

---

## distribution

```text
DistributionSpec {
  name: string
  params: map[string]string (optional)
}
```

Default:

```text
name: "uniform"
params: {}
```

Allowed names:

* `uniform`
* `normal`
* `skewEarly`
* `skewLate`
* `exponential`

Unknown names cause rejection.

Parameter validation must match CORE-SPEC.md.

---

## seed

```text
SeedSpec {
  strategy: "stable" | "daily" | "weekly"
  salt: string (optional)
}
```

Default:

```text
strategy: "stable"
salt: ""
```

Unknown strategy causes rejection.

---

## constraints

```text
ConstraintSpec {
  only: []ConstraintClause (optional)
  avoid: []ConstraintClause (optional)
}
```

Each clause corresponds to SYNTAX.md semantics.

Constraint schema must be validated structurally but deep semantic validation may occur in controller.

If constraints are unsatisfiable at runtime, period becomes `unschedulable`.

---

## policy

```text
PolicySpec {
  concurrency: "allow" | "forbid" | "replace"
  deadline: string (Go duration format)
  suspend: boolean
}
```

Defaults:

```text
concurrency: "forbid"
deadline: "0s"
suspend: false
```

Validation:

* deadline ≥ 0.
* Unknown concurrency value rejected.

---

## jobTemplate

Standard `batch/v1 JobTemplateSpec`.

Requirements:

* `spec.template.spec.restartPolicy` must be valid for Job.
* No fields may be mutated by controller except labels and owner references.

Controller must:

* Add owner reference to KronJob.
* Add deterministic labels:

  * `kron.io/name`
  * `kron.io/period-id`
  * `kron.io/chosen-time`

---

## Status

```text
KronJobStatus {
  observedGeneration: int64
  lastPeriodID: string
  lastNominalTime: string
  lastChosenTime: string
  lastOutcome: string
  nextPeriodID: string
  nextNominalTime: string
  nextChosenTime: string
  conditions: []Condition
}
```

All timestamps are RFC3339 UTC.

---

## lastOutcome

One of:

* `executed`
* `skipped`
* `missed`
* `unschedulable`

Empty if no period handled yet.

---

## next* Fields

Represent the next scheduled period as of last reconciliation.

Must reflect deterministic decision.

These fields are informational and may change if spec changes.

---

## Conditions

Standard Kubernetes Condition structure:

```text
Condition {
  type: string
  status: "True" | "False" | "Unknown"
  reason: string
  message: string
  lastTransitionTime: string
}
```

Defined condition types:

* `Ready`
* `InvalidSpec`
* `SchedulingError`
* `Unschedulable`

---

## Defaulting Rules

Defaults must be applied at admission or reconciliation:

* timezone → `UTC`
* window.mode → `after`
* window.duration → `0s`
* distribution.name → `uniform`
* seed.strategy → `stable`
* seed.salt → `""`
* policy.concurrency → `forbid`
* policy.deadline → `0s`
* policy.suspend → `false`

Defaulting must be stable within version.

---

## Immutability Rules

The following fields are immutable after creation:

* `spec.schedule`
* `spec.timezone`
* `spec.seed.strategy`

If changed:

* Controller must treat as new scheduling configuration.
* Future periods use new config.
* Already handled periods remain handled.

Alternatively, strict mode may reject mutation via validation webhook.

Immutability policy must be consistent within version.

---

## Reconciliation Semantics

Controller must:

1. Observe current time.
2. Compute decision for current period using kron-core.
3. Compare with status.
4. Enforce idempotency:

   * Do not create duplicate Job for same `period-id`.
5. Apply policy.
6. Update status atomically.

Reconciliation must be idempotent.

---

## Idempotency Rules

Before creating a Job:

* Check for existing Job with:

  * matching owner reference
  * matching `kron.io/period-id`

If exists:

* Do not create another.
* Update status accordingly.

---

## Suspension

If `policy.suspend=true`:

* No Jobs created.
* Periods are not marked handled.
* Status reflects suspended state.

When suspension lifted:

* Only future periods are considered.
* Missed periods follow deadline semantics.

---

## Deadline Handling

If `now > chosenTime + deadline`:

* Mark period `missed`.
* Do not create Job.
* Update status.

Deadline default `0s` means:

* If `now > chosenTime`, period is `missed`.

---

## Deletion Semantics

When KronJob is deleted:

* Owner reference ensures child Jobs are garbage collected.
* Controller must not create new Jobs.

Finalizers are optional but must not block deletion unnecessarily.

---

## Upgrade Rules

Within `v1alpha1`:

* Field names must not change.
* Status fields must not change meaning.
* Default values must remain stable.

Breaking changes require new API version (e.g., `v1beta1`).

Conversion webhook required if version upgrade introduces structural change.

---

## Validation Guarantees

CRD schema must enforce:

* Required fields present.
* Enum values constrained.
* Duration strings valid.
* Basic structural validation.

Deep semantic validation may occur in controller and reflected in `InvalidSpec` condition.

---

## Compatibility Guarantees

For identical spec and same Kron version:

* Same decisions must be produced.
* Same period-id labeling must occur.
* Same Job naming strategy must be used.

Behavioral changes require new API version.

---

## Naming of Jobs

Controller must name Jobs deterministically:

```text
<kronjob-name>-<period-hash>
```

`period-hash` derived from `period-id` truncated to safe length.

Name must:

* Be DNS-1123 compliant.
* Avoid collisions.

---

## Invariants

1. At most one Job per `(namespace, kronjob, period-id)`.
2. chosenTime always within window.
3. Status reflects last handled period.
4. Reconciliation is idempotent.
5. No duplicate Jobs on controller restart.
6. Spec changes affect only future periods.
7. Suspension prevents execution without losing configuration.

---

## Non-Goals

CRD does not:

* Provide workflow chaining.
* Provide retries.
* Provide backoff policies.
* Provide distributed locking beyond Job semantics.

KronJob defines scheduling, not orchestration.
