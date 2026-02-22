# Contributing to Kron

Thank you for your interest in contributing.

Kron prioritizes correctness, determinism, and long-term maintainability.

---

## Before Contributing

1. Read:
   - SPEC.md
   - CORE-SPEC.md
   - TEST-VECTORS.md
2. Ensure the change aligns with project principles.
3. Open an issue for significant changes before submitting a PR.

---

## Development Rules

- `kron-core` must remain pure and deterministic.
- No Kubernetes or OS dependencies in `core/`.
- No nondeterministic randomness.
- No hidden retries.
- No breaking behavior changes without version bump.

---

## Pull Request Requirements

- Tests required for all logic changes.
- Golden vectors must not change unless version increment justified.
- No unrelated refactoring.
- Keep changes minimal and focused.

---

## Commit Guidelines

- Use clear commit messages.
- Reference related issues.
- Avoid force-push after review begins.

---

## Reporting Bugs

Include:

- Version
- Configuration
- Expected behavior
- Actual behavior
- Logs (if applicable)

---

Kron values clarity over cleverness.