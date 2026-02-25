# Kron
[![codecov](https://codecov.io/github/desuex/kron/graph/badge.svg?token=8KYI8SHH91)](https://codecov.io/github/desuex/kron)
[![docs](https://readthedocs.org/projects/krontab/badge/?version=latest)](https://krontab.readthedocs.io/)



Deterministic probabilistic scheduling.

Kron is a scheduling system designed to eliminate synchronized execution spikes while preserving determinism and explainability.

It provides:

- Window-based scheduling
- Biased distributions (`uniform`, `skewEarly`, `skewLate`)
- Deterministic seed strategies
- Strict idempotency guarantees
- Transparent, human-readable decision logs
- A pure scheduling engine (`kron-core`)
- A host daemon (`krond`)
- A Kubernetes controller (`kron-operator`)

---

## Why Kron?

Traditional cron executes at exact timestamps.

Real systems suffer from:

- Load spikes (midnight storms)
- Predictable automation patterns
- Non-human execution timing
- Thundering herd effects

Kron spreads work safely, deterministically, and observably.

---

## Architecture

- `core/` – deterministic scheduling engine (`kron-core`)
- `cmd/` – CLI tools (`krontab`, `krond`, `kronctl`)

Planned components (`daemon/`, `operator/`) are documented but not implemented yet.

## Current Implementation (MVP)

- `core/`: deterministic seed hashing, SplitMix64 PRNG, window computation, bounded candidate sampling, constraint handling, and golden vectors (`v1`-`v7`)
- `cmd/krontab`: `lint`, `explain`, and `next` commands
- `daemon/cmd/krond`: early `start` command slice (config load, deterministic scheduling, execution, state persistence)
- Runtime distributions in `explain`/`next`: `uniform`, `skewEarly`, `skewLate`
- `normal` and `exponential` syntax is validated by `lint` but not executed by MVP runtime commands
- CI: `gofmt` check, `go vet`, tests, coverage threshold, and Sphinx docs build

## Implemented vs Planned

| Item | Status | Notes |
|---|---|---|
| `kron-core` deterministic engine | Implemented | Golden vectors (`v1`-`v7`) and 90%+ coverage |
| `krontab lint` | Implemented | Text/JSON output, strict validation |
| `krontab explain` | Implemented | Deterministic decision output, text/JSON |
| `krontab next` | Implemented | Deterministic multi-period preview |
| Runtime `normal`/`exponential` distributions | Planned | Currently lint-validated only |
| `krond` daemon | In progress | Early `start` slice implemented; full drop-in parity staged |
| `kronctl` helper CLI | Planned | Post-CLI MVP |
| Cross-platform release binaries | In progress | Milestone 4: Linux/macOS/Windows assets |

## Documentation

Start here:

| Guide | Description |
|---|---|
| [Read the Docs](https://krontab.readthedocs.io/) | Hosted documentation |
| [SETUP.md](docs/SETUP.md) | Local setup and build instructions |
| [USAGE.md](docs/USAGE.md) | MVP CLI usage (`lint`, `explain`, `next`) |
| [RELEASE.md](docs/RELEASE.md) | Freeze checklist, tag flow, and release artifact verification |
| [BENCHMARK.md](docs/BENCHMARK.md) | Performance and reliability benchmark gates for `krond` |

Reference specifications:

| Document | Description |
|---|---|
| [HELLOKRON.md](docs/HELLOKRON.md) | Project overview, goals, and architecture |
| [MANIFESTO.md](docs/MANIFESTO.md) | Design principles and early vision (historical) |
| [SPEC.md](docs/SPEC.md) | Scheduling and execution semantics (stable contract) |
| [CORE-SPEC.md](docs/CORE-SPEC.md) | `kron-core` engine formal contract |
| [SYNTAX.md](docs/SYNTAX.md) | Schedule syntax reference (cron + modifiers) |
| [KRONTAB.md](docs/KRONTAB.md) | `krond` configuration file format |
| [EXECUTION.md](docs/EXECUTION.md) | `krond` daemon execution model |
| [STATE.md](docs/STATE.md) | Persistent state model for `krond` |
| [ERROR-MODEL.md](docs/ERROR-MODEL.md) | Error categories and handling semantics |
| [LOGGING.md](docs/LOGGING.md) | Structured log format specification |
| [SECURITY.md](docs/SECURITY.md) | Security model and hardening requirements |
| [COMPAT.md](docs/COMPAT.md) | Compatibility with cron, systemd, CronJob |
| [CRON-DROPIN.md](docs/CRON-DROPIN.md) | Next-stage cron replacement compatibility profile |
| [TEST-VECTORS.md](docs/TEST-VECTORS.md) | Golden test vectors for determinism |
| [CRD-SPEC.md](docs/CRD-SPEC.md) | Kubernetes CRD schema and behavior |
| [CLI-SPEC.md](docs/CLI-SPEC.md) | CLI commands, flags, and exit codes |

---

## Status

Early development, active MVP delivery.

Milestone 1 (repository bootstrap) is complete.
Milestone 2 (core engine MVP) is complete.
Milestone 3 (CLI MVP) is complete.
Milestone 4 (MVP freeze) is in progress.

API and behavior are subject to change until v1.0.

---

## License

Apache License 2.0

See `LICENSE` for details.

---

## Design Principles

- Determinism first
- At most one execution per period
- No unbounded replay
- No implicit retries
- No hidden behavior
- Explicit over implicit

---

## Roadmap

See `ROADMAP.md`.

For the execution-focused minimal delivery track, see `MVP_PLAN.md`.

---

## Security

See `SECURITY.md`.

---

Kron aims to become a stable, predictable foundation for modern scheduling.
