# Kron

Deterministic probabilistic scheduling.

Kron is a scheduling system designed to eliminate synchronized execution spikes while preserving determinism and explainability.

It provides:

- Window-based scheduling
- Biased distributions (uniform, skewed, normal, exponential)
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
- `daemon/` – host execution daemon (`krond`)
- `operator/` – Kubernetes controller (`kron-operator`)
- `cmd/` – CLI tools (`krontab`, `krond`, `kronctl`)

## Documentation

Start here:

| Document | Description |
|---|---|
| [HELLOKRON.md](docs/HELLOKRON.md) | Project overview, goals, and architecture |
| [MANIFESTO.md](docs/MANIFESTO.md) | Design principles and early vision (historical) |
| [SPEC.md](docs/SPEC.md) | Scheduling and execution semantics (stable contract) |
| [CORE-SPEC.md](docs/CORE-SPEC.md) | `kron-core` engine formal contract |
| [SYNTAX.md](docs/SYNTAX.md) | Schedule syntax reference (cron + modifiers) |
| [EXECUTION.md](docs/EXECUTION.md) | `krond` daemon execution model |
| [STATE.md](docs/STATE.md) | Persistent state model for `krond` |
| [ERROR-MODEL.md](docs/ERROR-MODEL.md) | Error categories and handling semantics |
| [LOGGING.md](docs/LOGGING.md) | Structured log format specification |
| [SECURITY.md](docs/SECURITY.md) | Security model and hardening requirements |
| [COMPAT.md](docs/COMPAT.md) | Compatibility with cron, systemd, CronJob |
| [TEST-VECTORS.md](docs/TEST-VECTORS.md) | Golden test vectors for determinism |
| [CRD-SPEC.md](docs/CRD-SPEC.md) | Kubernetes CRD schema and behavior |
| [CLI-SPEC.md](docs/CLI-SPEC.md) | CLI commands, flags, and exit codes |

---

## Status

Early development.

Core specifications are defined.
Implementation is in progress.

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

---

## Security

See `SECURITY.md`.

---

Kron aims to become a stable, predictable foundation for modern scheduling.