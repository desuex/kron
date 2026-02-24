# Repository Structure

This file describes the current repository layout and the intended direction for MVP work.

## Current Layout (as implemented)

```text
kron/
  .github/workflows/ci.yml
  .readthedocs.yaml
  CHANGELOG.md
  MVP_PLAN.md
  ROADMAP.md
  STRUCTURE.md
  TODO.md
  go.work

  docs/
    SETUP.md
    USAGE.md
    index.md
    conf.py
    requirements.txt
    *.md specs

  core/
    go.mod
    pkg/core/
      decision.go
      errors.go
      prng.go
      seed.go
      types.go
      vectors_test.go
      *_test.go
    testdata/vectors/
      v1.json ... v7.json

  cmd/krontab/
    go.mod
    main.go
    lint.go
    config.go
    constraints.go
    cron.go
    *_test.go
    integration_test.go

  daemon/
    go.mod
    README.md
    pkg/daemon/
      dependency.go
      doc.go

  operator/
    go.mod
    README.md
    pkg/operator/
      dependency.go
      doc.go

  scripts/
    ci.sh
    coverage.sh
    test.sh
```

## Responsibility Boundaries

- `core/`: deterministic scheduling logic and vector-backed behavior.
- `cmd/krontab/`: local CLI interface and config parsing for MVP workflows.
- `daemon/`: host adapter scaffold (implementation pending).
- `operator/`: Kubernetes adapter scaffold (implementation pending).
- `docs/`: specs, onboarding, and reference documentation.
- `scripts/`: local and CI helper scripts.

## Near-Term Direction

- Keep `core` free from adapter concerns.
- Continue CLI parity work in `cmd/krontab`.
- Add daemon/operator implementation only after CLI MVP freeze criteria are met.
