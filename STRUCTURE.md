kron/
  README.md
  LICENSE
  SECURITY.md
  CODE_OF_CONDUCT.md
  CONTRIBUTING.md
  GOVERNANCE.md
  ROADMAP.md
  CHANGELOG.md

  docs/
    HELLOKRON.md
    SYNTAX.md
    LOGS.md
    EXECUTION.md
    SPEC.md
    CORE-SPEC.md
    STATE.md
    ERROR-MODEL.md
    SECURITY.md
    COMPAT.md
    TEST-VECTORS.md
    CRD-SPEC.md
    CLI-SPEC.md

  core/                         # kron-core (pure engine)
    go.mod
    go.sum
    README.md
    pkg/
      core/
        decision.go             # DecisionRequest/Result types
        window.go               # window computation
        seed.go                 # seed + hashing (SHA-256)
        prng.go                 # SplitMix64 + NextFloat64
        distribution/
          uniform.go
          skew_early.go
          skew_late.go
          normal.go             # optional later
          exponential.go        # optional later
        constraints/
          model.go
          eval.go
          parse.go              # if you parse constraint clauses here
        errors.go               # typed errors per ERROR-MODEL
        vectors.go              # helpers for golden vectors
    testdata/
      vectors/
        v1.json
        v2.json
        ...
    internal/
      tzdb/                     # optional: pin timezone source policy

  cmd/                          # binaries live at repo root, depend on modules below
    krond/
      main.go
    krontab/
      main.go
    kronctl/
      main.go                   # optional; can arrive later

  daemon/                       # krond implementation (adapter)
    go.mod
    go.sum
    README.md
    pkg/
      daemon/
        daemon.go               # lifecycle
        scheduler.go            # priority queue, timers
        runner/
          runner.go             # fork/exec + supervision
          signals.go
          timeout.go
        state/
          state.go              # schema structs
          store.go              # atomic writes, fsync
          migrate.go
        config/
          load.go
          model.go              # parsed config structs
        logging/
          text.go               # LOGS.md format writer
          json.go
        limits/
          rlimits.go            # optional
    testdata/
      configs/
      state/

  operator/                     # kubernetes controller (adapter)
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
        decision.go             # map CRD -> core DecisionRequest
        jobs.go                 # create Job w/ labels/annotations
      logging/
        text.go
        json.go
      metrics/
        metrics.go
    config/                     # kubebuilder/kustomize assets
      crd/
      rbac/
      manager/
      samples/
    charts/
      kron/                     # Helm chart (optional but valuable)
    hack/
      kind-e2e.sh

  tooling/
    golangci-lint.yml
    goreleaser/
      .goreleaser.yaml
    cosign/
    sbom/

  scripts/
    generate.sh                 # codegen, manifests, etc.
    test.sh
    e2e.sh

  .github/
    workflows/
      ci-core.yml
      ci-daemon.yml
      ci-operator.yml
      release.yml