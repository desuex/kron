# MVP Plan

This document tracks the minimum delivery path for Kron.

## Snapshot (2026-02-24)

- Milestone 1: completed
- Milestone 2: completed
- Milestone 3: in progress
- Milestone 4: pending

## MVP Goal

Ship a deterministic scheduling engine with a usable CLI and reproducible outputs.

In scope:
- `core` deterministic scheduler (`kron-core`)
- `cmd/krontab` commands: `lint`, `explain`, `next`
- Golden vectors and deterministic tests
- CI enforcement for formatting, vet, tests, and coverage

Out of scope for MVP:
- `krond` execution loop
- Kubernetes controller features
- Advanced distributions beyond implemented MVP set
- Production observability/hardening program

## Milestone 1: Repository Bootstrap

Status: completed

Delivered:
- Monorepo layout and module wiring
- Baseline CI
- Initial specs in `docs/`

Exit criteria: met.

## Milestone 2: Core Engine MVP

Status: completed

Delivered in `core`:
- deterministic seed derivation and period-key strategies
- SplitMix64 PRNG
- window modes: `after`, `before`, `center` (`around` alias supported at CLI layer)
- distributions: `uniform`, `skewEarly`, `skewLate`
- deterministic sampling with bounded attempts
- constraint evaluation with unschedulable handling
- typed error model for invalid core inputs
- golden vector coverage (`core/testdata/vectors/v1.json` through `v7.json`)

Exit criteria: met.

## Milestone 3: CLI MVP (`krontab`)

Status: in progress

Current capabilities:
- `krontab lint --file <path> [--format text|json]`
- `krontab explain <job> --at <RFC3339> [--file <path>] [--format text|json]`
- `krontab next <job> --file <path> [--count N] [--at <RFC3339>] [--format text|json]`
- config-driven timezone/seed/constraint behavior in `explain` and `next`

Remaining work:
1. Close remaining syntax parity gaps against `docs/SYNTAX.md`.
2. Expand integration/snapshot coverage for CLI output stability.
3. Keep error messages and exit code behavior aligned with `docs/ERROR-MODEL.md` and `docs/CLI-SPEC.md`.

Exit criteria:
- deterministic outputs for repeated invocations with identical inputs
- stable text/json contracts covered by tests

## Milestone 4: MVP Freeze

Status: pending

Tasks:
- finalize MVP scope statement in docs/README
- tag pre-release (`v0.1.0-alpha.1`)
- publish quickstart and implemented-vs-planned matrix

Exit criteria:
- new contributor can run setup, lint, explain, and next in under five minutes

## Current Risks

- Syntax breadth can expand faster than test coverage.
  - Control: merge syntax features only with deterministic tests.
- CLI behavior can drift from specs during rapid iteration.
  - Control: treat docs and tests as the compatibility gate.
- Docs quality can regress with new content.
  - Control: keep Sphinx warnings as CI failures.
