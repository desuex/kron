# Repository Structure

```
kron/
  README.md
  LICENSE
  SECURITY.md
  CODE_OF_CONDUCT.md
  CONTRIBUTING.md
  GOVERNANCE.md
  ROADMAP.md
  CHANGELOG.md

  docs/                           # specifications and design documents
    MANIFESTO.md
    HELLOKRON.md
    SPEC.md
    CORE-SPEC.md
    SYNTAX.md
    KRONTAB.md
    EXECUTION.md
    STATE.md
    ERROR-MODEL.md
    LOGGING.md
    SECURITY.md
    COMPAT.md
    TEST-VECTORS.md
    CRD-SPEC.md
    CLI-SPEC.md

  core/                           # pure deterministic scheduling engine
    go.mod
    go.sum
    README.md
    pkg/
      core/
        decision.go
        window.go
        seed.go
        prng.go
        distribution/
          uniform.go
          skew_early.go
          skew_late.go
          normal.go
          exponential.go
        constraints/
          model.go
          eval.go
          parse.go
        errors.go
        vectors.go
    testdata/
      vectors/
        v1.json
        v2.json
        ...
    internal/
      tzdb/

  cmd/                            # binary entrypoints
    krond/
      main.go
    krontab/
      main.go
    kronctl/
      main.go

  daemon/                         # host daemon adapter
    go.mod
    go.sum
    README.md
    pkg/
      daemon/
        daemon.go
        scheduler.go
        runner/
          runner.go
          signals.go
          timeout.go
        state/
          state.go
          store.go
          migrate.go
        config/
          load.go
          model.go
        logging/
          text.go
          json.go
        limits/
          rlimits.go
    testdata/
      configs/
      state/

  operator/                       # Kubernetes controller adapter
    go.mod
    go.sum
    README.md
    api/
      v1alpha1/
        kronjob_types.go
        groupversion_info.go
    controllers/
      kronjob_controller.go
    pkg/
      adapters/
        decision.go
        jobs.go
      logging/
        text.go
        json.go
      metrics/
        metrics.go
    config/
      crd/
      rbac/
      manager/
      samples/
    charts/
      kron/
    hack/
      kind-e2e.sh

  tooling/                        # build and release configuration
    golangci-lint.yml
    goreleaser/
      .goreleaser.yaml
    cosign/
    sbom/

  scripts/                        # development and CI scripts
    generate.sh
    test.sh
    e2e.sh

  .github/
    workflows/
      ci-core.yml
      ci-daemon.yml
      ci-operator.yml
      release.yml
```
