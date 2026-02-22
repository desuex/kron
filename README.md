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

- `core/` – deterministic scheduling engine
- `daemon/` – host execution daemon
- `operator/` – Kubernetes controller
- `cmd/` – CLI tools

Full specifications live in `/docs`.

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