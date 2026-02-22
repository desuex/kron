# Kron manifesto (project principles)

**Kron is probabilistic scheduling for Kubernetes.**
It exists to make automation look less like a metronome and more like reality, while keeping operations safe, explainable, and controllable.

### What we believe

1. **Determinism beats surprise.**
   Randomness must be *controlled*: reproducible per job/run, explainable, and debuggable.
2. **Scheduling is a reliability feature.**
   Reducing synchronized load spikes is as important as retries, rate limits, and backpressure.
3. **Time is a domain, not a timestamp.**
   “Run around 10am” is a legitimate requirement. “Exactly 10:00:00” is usually a bug.
4. **Safe by default, powerful by configuration.**
   Conservative defaults. No foot-guns like “infinite catch-up storms.”
5. **Observable or it didn’t happen.**
   Every chosen fire time should be explainable (“why did it run at 10:37?”).
6. **Kubernetes-native, not Kubernetes-adjacent.**
   CRDs, RBAC-minimal controller, Jobs created like a good citizen, works with GitOps.
7. **Boring tech wins.**
   Prefer simple, testable components over novelty. Kron should be easy to run for 5 years.

### The promise

Kron will provide:

* load smoothing (anti “cron storm”)
* human-like time variability (distribution bias)
* gentle unpredictability for automation
* with reproducible decisions, auditability, and strong operational controls

---

# Product shape (what Kron *is*)

**A controller + CRD** that creates normal Kubernetes `Job`s (or `CronJob`s) at computed times.

### CRD sketch: `KronJob` (name flexible)

Core fields:

* `schedule`: cron expression (baseline intent)
* `window`: allowable offset range (e.g., `45m`, `2h`)
* `distribution`: `uniform | normal | skewEarly | skewLate | exponential | custom` (start with a small set)
* `mode`: `jitterAroundSchedule` vs `pickTimeInWindow`
* `seed`: deterministic keying strategy (`jobName`, `namespace`, `date`, optional salt)
* `timezone`: explicit TZ support
* `concurrencyPolicy`: `Allow|Forbid|Replace` (like CronJob)
* `startingDeadlineSeconds`: bound “missed run” catch-up
* `suspend`: bool
* `jobTemplate`: same idea as `CronJob.spec.jobTemplate`

**Important design choice:**
Default to **deterministic randomness** (seeded) so runs are spread but stable across controller restarts.

---

# Roadmap

## Phase 0 — “It builds, it runs” (Week 1–2 worth of effort)

* Repo scaffold via Kubebuilder
* CRD: minimal `schedule`, `window`, `jobTemplate`
* Controller: reconciles, computes nextRun, creates Job, updates status
* Status fields:

  * `lastScheduleTime`
  * `nextScheduleTime`
  * `lastChosenTime` + `chosenBy` (seed + distribution)
* Helm chart + example manifests
* Docs: “why Kron exists”, quickstart, FAQ

Deliverable: a working alpha that spreads CronJobs across a window (uniform).

## Phase 1 — Determinism + operational safety (MVP)

* Seeded RNG (stable, explainable)
* Concurrency policies (Forbid/Replace)
* Missed run semantics:

  * “skip if late”
  * bounded catch-up (no storms)
* Leader election
* Events + metrics:

  * `kron_next_run_timestamp`
  * `kron_jobs_scheduled_total`
  * `kron_schedule_decision_seconds` (latency)
* `kubectl describe` friendly: clear reasons for timing decisions

Deliverable: production-shaped MVP.

## Phase 2 — Your differentiator: distribution bias

Add distributions that matter for your 3 pains:

* `uniform`: spread load
* `skewEarly` / `skewLate`: human-ish behavior (tendency to be late, etc.)
* `normal` (bounded/truncated): clustered around target time
* `exponential` (bounded): “usually near start, sometimes later” (or vice versa)

Also add:

* `constraints`: “don’t run during these hours”, business hours, weekends, etc.
* `calendar`: optional exclusions (maintenance windows)

Deliverable: the thing that’s *not* just jitter.

## Phase 3 — Enterprise features (adoption unlockers)

* RBAC minimized + documented
* Multi-namespace watch option
* Multi-arch images, signed images (cosign)
* Admission/validation:

  * CRD schema validation
  * optional webhook for “safe settings” linting
* GitOps-friendly:

  * immutable fields policy
  * predictable status updates
* Compatibility story with existing CronJobs (migration guide)

Deliverable: “security team won’t block it” release.

## Phase 4 — “Become that guy” maintainership mode

* SemVer, release notes, upgrade notes
* conformance test suite (kind-based)
* performance + scale testing (10k schedules)
* clear governance:

  * CODEOWNERS
  * CONTRIBUTING
  * issue templates
* stability policy: what is guaranteed, what is experimental
* optional: CNCF Sandbox pitch (only if you want that life)

Deliverable: mature project with contributors.

---

# Repository layout suggestion

* `/api/v1alpha1` – types
* `/controllers` – reconciler
* `/pkg/scheduler` – pure scheduling logic (unit-test heavy)
* `/charts/kron` – Helm
* `/config` – kustomize assets
* `/docs` – rationale, semantics, examples

Key idea: keep the scheduling algorithm as a **pure function**:
`(schedule, window, distribution, seed, now) -> nextTime`
That makes testing and trust way easier.
