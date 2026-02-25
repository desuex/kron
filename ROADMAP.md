# ROADMAP.md

## Vision

Kron becomes the reference implementation of deterministic, probabilistic scheduling:

* A production-grade Kubernetes controller.
* A host-level daemon for non-cluster environments.
* A stable, pure scheduling engine (`kron-core`) trusted by both.

Correctness, determinism, and operational safety take priority over feature velocity.

---

## Progress Snapshot (2026-02-25)

* Phase 0 (Repository Bootstrap): completed.
* Phase 1 (kron-core): completed.
* Phase 2 (krontab): completed.
* MVP freeze track: in progress.
* `core` deterministic engine MVP is implemented and validated with golden vectors (`v1`-`v7`).
* `krontab` implements `lint`, `explain`, and `next`.
* `krond` initial `start` runtime slice is implemented (config load, deterministic scheduling, execution, state persistence).
* CI enforces format, vet, tests, docs build, and 90% combined coverage.
* Release workflow builds tagged `krontab` binaries for Linux/macOS/Windows with checksums.
* Freeze checklist and tag runbook are documented in `docs/RELEASE.md`.
* Cron drop-in compatibility profile is documented in `docs/CRON-DROPIN.md`.

---

# Phase 0 — Repository Bootstrap

## Goals

* Establish structure.
* Freeze core contracts.
* Prevent architectural drift.

## Deliverables

* Monorepo structure:

  * `core/`
  * `daemon/`
  * `operator/`
  * `cmd/`
  * `docs/`
* All spec documents committed:

  * HELLOKRON.md
  * SPEC.md
  * CORE-SPEC.md
  * SYNTAX.md
  * STATE.md
  * ERROR-MODEL.md
  * SECURITY.md
  * COMPAT.md
  * TEST-VECTORS.md
  * CRD-SPEC.md
  * CLI-SPEC.md
* CI skeleton:

  * lint
  * build
  * test
* Go modules initialized:

  * `core` independent
  * `daemon` depends on `core`
  * `operator` depends on `core`

## Exit Criteria

* Repository builds.
* No circular dependencies.
* `core` imports no Kubernetes or OS packages.
* CI passes with bootstrap implementations.

## Status

Completed.

---

# Phase 1 — kron-core (Engine Foundation)

## Status

Completed.
Delivered: seed derivation, SplitMix64 PRNG, window calculation, implemented MVP distributions (`uniform`, `skewEarly`, `skewLate`), bounded candidate sampling, constraint evaluation, deterministic decision output, and 90%+ core coverage.

## Goals

Implement deterministic decision engine exactly as specified.

## Work

* Seed derivation (SHA-256)
* SplitMix64 PRNG
* Window calculation
* Distribution implementations:

  * uniform
  * skewEarly
  * skewLate
* Constraint evaluation (basic)
* Candidate sampling loop
* DecisionResult struct
* Error types

## Testing

* Implement all vectors in `TEST-VECTORS.md`
* Byte-for-byte comparison of:

  * SeedHash
  * WindowStart/End
  * ChosenTime
* Determinism test:

  * Same inputs across multiple runs
* Cross-platform test matrix in CI

## Exit Criteria

* All golden vectors pass.
* 90%+ test coverage in `core`.
* No nondeterministic failures in CI.
* Public API stable.

## Completion Notes

* Golden vectors are implemented and passing (`core/testdata/vectors/v1.json` through `v7.json`).
* Deterministic behavior is covered by unit tests and vector tests.
* Remaining work moves to CLI parity and downstream adapters.

---

# Phase 2 — krontab (CLI Interface to Core)

## Status

Completed.
Delivered so far: `krontab lint`, `krontab explain`, `krontab next`, text/json outputs, snapshot tests, expanded integration tests, and parity coverage across supported vector families.

## Goals

Expose deterministic scheduling to users.

## Work

* `krontab lint`
* `krontab explain`
* `krontab next`
* Config file parser (matching SYNTAX.md)
* Text + JSON output modes
* Exit code contract enforcement

## Testing

* CLI integration tests.
* Snapshot testing for explain output.
* Deterministic output verification.

## Exit Criteria

* `krontab explain` reproduces TEST-VECTORS.
* No daemon required.
* Output stable across runs.
* Milestone 4 freeze includes cross-platform `krontab` release artifacts (Linux, macOS, Windows).

---

# Phase 3 — krond (Host Daemon MVP + Cron Drop-in Track)

## Goals

Deliver a minimal but correct execution daemon and a defined cron drop-in compatibility profile.

## Work

* Scheduler loop (priority queue)
* State persistence layer
* Atomic writes
* Lock enforcement
* Fork/exec runner
* Cron source adapters:

  * `/etc/crontab`
  * `/etc/cron.d/*`
  * exported user crontab inputs
* Compatibility matrix implementation from `docs/CRON-DROPIN.md`
* Concurrency policies:

  * allow
  * forbid
* Deadline handling
* Structured logging per LOGGING.md
* Migration documentation and compatibility test corpus

## Exclusions

* No retry system.
* No advanced resource limits.
* No plugin system.
* No full bug-for-bug parity with all cron variants.
* No `run-parts` / `MAILTO` parity in this phase.

## Testing

* Integration tests with temporary state directory.
* Crash recovery simulation.
* Duplicate execution prevention tests.
* Deadline edge case tests.
* Cron compatibility corpus for Tier 1/Tier 2 features.

## Exit Criteria

* No duplicate execution across restart.
* State file integrity guaranteed.
* Concurrency policies enforced.
* Deterministic decision preserved.
* Tier 1 cron drop-in capabilities are green and documented.

---

# Phase 4 — Distribution Expansion

## Goals

Complete core distribution set.

## Work

* normal distribution
* exponential distribution
* distribution parameter validation
* constraint edge-case coverage

## Testing

* New golden vectors.
* Constraint-heavy unschedulable cases.
* Statistical sanity tests (distribution shape validation).

## Exit Criteria

* All distributions deterministic.
* Sampling bounded.
* Unschedulable behavior correct.

---

# Phase 5 — Kubernetes Operator (Alpha)

## Goals

Deliver `KronJob` controller with minimal viable feature set.

## Work

* Kubebuilder scaffolding
* CRD schema implementation
* Reconciler
* Job creation logic
* Idempotency checks via labels
* Status updates
* Conditions implementation
* Leader election
* RBAC minimal rules
* Sample manifests

## Testing

* envtest controller tests
* kind-based e2e tests
* Duplicate Job prevention
* Deadline handling
* Suspension behavior

## Exit Criteria

* At most one Job per period.
* Status reflects decisions.
* Restart-safe.
* Works in kind cluster.

---

# Phase 6 — Observability & Hardening

## Goals

Make Kron production-ready.

## Work

* Prometheus metrics
* JSON log mode parity
* Structured log verification
* Security audit:

  * state file permissions
  * privilege drop verification
* Container image hardening:

  * non-root
  * minimal base image
* SBOM generation
* Release signing
* Helm chart polish

## Exit Criteria

* Security.md compliance verified.
* Metrics exposed and documented.
* Release artifacts signed.
* Example production deployment documented.

---

# Phase 7 — Advanced Safety Features

## Goals

Enterprise-readiness improvements.

## Work

* Immutable spec enforcement (optional webhook)
* Admission validation for CRD
* Sampling budget tuning
* Configurable history retention
* Operator multi-namespace mode
* Performance testing (thousands of jobs)

## Exit Criteria

* Stable behavior under 10k schedules.
* No O(n²) performance paths.
* Memory usage linear in job count.

---

# Phase 8 — v1beta1 API Stabilization

## Goals

Lock core semantics.

## Work

* API review of CRD
* Deprecation warnings if needed
* Compatibility test suite expansion
* Versioned API conversion logic (if needed)

## Exit Criteria

* API stability guarantees formalized.
* Breaking changes require new version.
* Clear migration documentation.

---

# Phase 9 — Ecosystem Integration

## Goals

Make Kron visible and usable.

## Work

* GitHub README polish
* Example use cases:

  * load smoothing
  * human-like messaging
  * home automation
* Benchmark document
* Blog post: “Cron Storms in Kubernetes”
* Sample dashboards
* Optional CNCF sandbox exploration

## Exit Criteria

* Public announcement ready.
* Reproducible demo environment.
* Clear differentiation vs CronJob and systemd.

---

# Long-Term Objectives

* Maintain strict determinism across versions.
* Maintain stable golden test vectors.
* Avoid feature creep into workflow engine territory.
* Prioritize correctness over convenience.
* Maintain small, understandable core.

---

# Non-Goals

Kron will not become:

* A workflow orchestration engine.
* A distributed queue.
* A replacement for Airflow.
* A retry framework.
* A general automation platform.

Kron remains a deterministic probabilistic scheduler.

---

# Stability Milestones

* v0.1 — core + krontab working.
* v0.2 — krond stable.
* v0.3 — operator alpha.
* v0.5 — production-safe daemon.
* v1.0 — stable API, deterministic guarantees frozen.

---

# Success Criteria

Kron succeeds when:

* Users rely on deterministic spread scheduling in production.
* Golden vectors never regress.
* No duplicate execution bugs are reported.
* The codebase remains small and understandable.
* Behavior is predictable enough that logs can fully explain any execution.

Correctness and determinism define the project.
