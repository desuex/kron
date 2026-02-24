# MVP_PLAN

Simple implementation plan for Kron, based on current repository state.

## Current State

- Specifications exist and are detailed in `docs/`.
- `core/` is implemented for deterministic seed/PRNG/window/uniform decisions.
- `cmd/krontab` implements `lint`, `explain`, and `next`.
- `daemon/` and `operator/` module scaffolds are present and wired to `core`.
- CI runs format checks, `go vet`, tests, and a 90% combined coverage threshold.

## Milestone Status (2026-02-24)

- Milestone 1 (Repository Bootstrap): completed.
- Milestone 2 (Core Engine MVP): in progress.
- Milestone 3 (CLI MVP): in progress.
- Milestone 4 (MVP Freeze): pending.

## MVP Scope (Simple)

Build only what is needed to prove the core value: deterministic probabilistic scheduling with a usable local CLI.

In scope:
- `kron-core` minimal engine
- `krontab` minimal CLI (`lint`, `explain`, `next`)
- Golden-vector determinism tests

Out of scope for MVP:
- `krond` daemon execution loop
- Kubernetes operator (`kron-operator`)
- Advanced distributions (`normal`, `exponential`)
- Full observability and hardening work

## Milestone 1: Repository Bootstrap (1-2 days)

Status: completed.

Deliverables:
- Create initial directories:
  - `core/`
  - `cmd/krontab/`
  - `scripts/`
  - `.github/workflows/`
- Initialize Go modules:
  - `core/go.mod`
  - root or workspace wiring for `cmd/krontab`
- Add baseline CI:
  - build
  - unit test
  - `go vet`

Exit criteria:
- module-aware test targets pass on clean checkout.
- CI runs green with coverage threshold enforcement.

## Milestone 2: Core Engine MVP (4-6 days)

Status: in progress.

Implement in `core`:
- Types for job spec, period, window, decision (implemented)
- Seed derivation (SHA-256) (implemented)
- SplitMix64 PRNG (implemented)
- Window modes (`before`, `after`, `center`) (implemented)
- Distributions:
  - `uniform` (implemented)
  - `skewEarly`
  - `skewLate`
- Candidate sampling loop with bounded attempts
- Basic constraints support from `docs/CORE-SPEC.md`

Testing:
- Port vectors from `docs/TEST-VECTORS.md` for implemented distributions
- Determinism test (same input -> exact same decision)
- Boundary tests for window and deadline behavior

Exit criteria:
- All implemented vectors pass byte-for-byte.
- Determinism tests stable across repeated runs.

## Milestone 3: CLI MVP (`krontab`) (2-3 days)

Status: in progress.

Commands:
- `krontab lint --file <path>` (implemented)
- `krontab explain <job> --at <RFC3339> [--format text|json]` (implemented)
- `krontab next <job> --count N` (implemented)

Behavior:
- Parse minimal config form from `docs/KRONTAB.md`
- Return stable, explicit errors (aligned with `docs/ERROR-MODEL.md`)
- Output decision internals required for reproducibility

Testing:
- CLI integration tests for success/failure exit codes (implemented for `next`)
- Snapshot tests for text and json outputs

Exit criteria:
- `krontab explain` reproduces same decision for same inputs.
- Output contract stable under repeated runs.

## Milestone 4: MVP Freeze (1 day)

Status: pending.

- Tag pre-release (`v0.1.0-alpha.1`)
- Publish:
  - quickstart in `README.md`
  - implemented subset vs full roadmap matrix

Exit criteria:
- New contributor can run lint/explain locally in under 5 minutes.

## Immediate Next Tasks (Do First)

1. Implement `skewEarly` and `skewLate` distributions in `core`.
2. Add and pass golden vectors for implemented distribution/mode combinations.
3. Tighten `krontab` config parsing toward `SYNTAX.md` parity.
4. Add snapshot tests for `explain` and `next` output stability.

## Risks and Controls

- Risk: spec breadth slows delivery.
  - Control: enforce strict MVP scope and defer daemon/operator.
- Risk: non-deterministic behavior from time parsing/randomness.
  - Control: deterministic test fixtures and exact golden outputs.
- Risk: parsing complexity in full KRONTAB.
  - Control: support minimal subset first and fail clearly on unsupported fields.
