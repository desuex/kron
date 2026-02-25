# Changelog

All notable changes to this project will be documented in this file.

The format is inspired by Keep a Changelog.
The project adheres to Semantic Versioning.

---

## [Unreleased]

### Added
- `kron-core` golden vector suite expanded through `v7.json`.
- Read the Docs integration files (`.readthedocs.yaml`, `docs/conf.py`, docs requirements).
- `docs/SETUP.md` and `docs/USAGE.md` onboarding guides.
- CI docs build step (`sphinx-build -n -W`) in GitHub Actions.
- Codecov upload in CI with explicit coverage file configuration.

### Changed
- Milestone 2 (Core Engine MVP) status moved to completed.
- Core decision engine supports deterministic `skewEarly` / `skewLate` with optional skew shape input.
- Constraint coverage expanded (`hours`, `dow`, `dom`, `months`, `between`, `date`, `dates`).
- README and planning documents updated to reflect current MVP scope and progress.

### Deprecated

### Removed

### Fixed
- Read the Docs build warnings from invalid fenced-code info strings in spec docs.
- Read the Docs heading-level warnings in `docs/MANIFESTO.md`.
- Go cache warning in CI by setting explicit dependency paths for `actions/setup-go`.

### Security

---

## [v0.1.0-alpha.1] - 2026-02-25 (planned)

### Added
- `krontab` MVP commands: `lint`, `explain`, `next`.
- `kron-core` deterministic engine coverage with golden vectors (`v1`-`v7`).
- Release workflow for tagged cross-platform `krontab` binaries and `SHA256SUMS`.
- Release runbook and freeze checklist (`docs/RELEASE.md`).

### Changed
- CLI parity tests expanded across supported vector families.
- MVP docs aligned with current runtime distribution scope.

### Fixed
- Read the Docs and Sphinx strict-build issues addressed for MVP docs set.
