# MVP_PLAN

Simple implementation plan for Kron, based on current repository state (docs complete, code not started).

## Current State

- Specifications exist and are detailed in `docs/`.
- No implementation directories currently exist (`core/`, `cmd/`, `daemon/`, `operator/`).
- `TODO.md` is still focused on spec-review tasks.

## MVP Scope (Simple)

Build only what is needed to prove the core value: deterministic probabilistic scheduling with a usable local CLI.

In scope:
- `kron-core` minimal engine
- `krontab` minimal CLI (`lint`, `explain`)
- Golden-vector determinism tests

Out of scope for MVP:
- `krond` daemon execution loop
- Kubernetes operator (`kron-operator`)
- Advanced distributions (`normal`, `exponential`)
- Full observability and hardening work

## Milestone 1: Repository Bootstrap (1-2 days)

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
- `go test ./...` passes on clean checkout.
- CI runs green with scaffolded code.

## Milestone 2: Core Engine MVP (4-6 days)

Implement in `core`:
- Types for job spec, period, window, decision
- Seed derivation (SHA-256)
- SplitMix64 PRNG
- Window modes (`before`, `after`, `center`)
- Distributions:
  - `uniform`
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

Commands:
- `krontab lint --file <path>`
- `krontab explain <job> --at <RFC3339> [--format text|json]`

Behavior:
- Parse minimal config form from `docs/KRONTAB.md`
- Return stable, explicit errors (aligned with `docs/ERROR-MODEL.md`)
- Output decision internals required for reproducibility

Testing:
- CLI integration tests for success/failure exit codes
- Snapshot tests for text and json outputs

Exit criteria:
- `krontab explain` reproduces same decision for same inputs.
- Output contract stable under repeated runs.

## Milestone 4: MVP Freeze (1 day)

- Tag pre-release (`v0.1.0-alpha.1`)
- Publish:
  - quickstart in `README.md`
  - implemented subset vs full roadmap matrix

Exit criteria:
- New contributor can run lint/explain locally in under 5 minutes.

## Immediate Next Tasks (Do First)

1. Create the bootstrap directory/module layout.
2. Add minimal CI workflow for build/test.
3. Implement seed + PRNG + uniform distribution with tests.
4. Implement `krontab explain` wired to core decision path.

## Risks and Controls

- Risk: spec breadth slows delivery.
  - Control: enforce strict MVP scope and defer daemon/operator.
- Risk: non-deterministic behavior from time parsing/randomness.
  - Control: deterministic test fixtures and exact golden outputs.
- Risk: parsing complexity in full KRONTAB.
  - Control: support minimal subset first and fail clearly on unsupported fields.
