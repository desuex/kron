# TODO

MVP-only backlog for Milestone 3 (CLI MVP) and Milestone 4 (MVP freeze).
Last updated: 2026-02-25.

## Completed Recently

- [x] Implement `krontab` MVP commands: `lint`, `explain`, `next`.
- [x] Add golden vectors in `core` (`v1` through `v7`) and keep core coverage above 90%.
- [x] Enforce CI quality gates for format, vet, tests, coverage threshold, and docs build.
- [x] Add snapshot tests for stable `krontab explain` / `krontab next` output.
- [x] Expand integration tests for modifier combinations and pessimistic paths.
- [x] Add strict runtime parsing for `@policy(...)` in `explain`/`next` config loading.
- [x] Reconcile `docs/CLI-SPEC.md` with current MVP command flags and runtime behavior.
- [x] Resolve distribution scope for MVP runtime: `normal`/`exponential` are lint-validated but not executed by `explain`/`next`.
- [x] Add vector-parity CLI tests across supported `core/testdata/vectors` families.
- [x] Freeze canonical stderr text and exit-code expectations in tests and `docs/CLI-SPEC.md`.
- [x] Add an explicit implemented-vs-planned matrix in `README.md`.
- [x] Add release workflow that builds `krontab` binaries for Linux/macOS/Windows and publishes checksums on tags.
- [x] Add initial `krond` benchmark harness (`go test -bench`) and `scripts/bench.sh` report wrapper.
- [x] Add initial `krond` benchmark threshold check script and wire it in CI (non-blocking).
- [x] Enforce `krond` runtime concurrency behavior for `allow|forbid`.
- [x] Enforce single-instance `krond` runtime lock via state-dir lock file.

## Milestone 3 Blockers (Must Finish)

- [x] Expand CLI vector parity beyond selected cases to cover all currently supported vector families.
- [x] Final pass on docs/examples to ensure no MVP command examples imply unimplemented runtime behavior.

## Documentation and Quality

- [ ] Keep Read the Docs warning-free (`sphinx-build -n -W`) on every main-branch change.
- [ ] Keep local `scripts/ci.sh` and GitHub Actions quality gates aligned.
- [ ] Promote benchmark gate checks in CI from non-blocking to blocking after Linux baseline stabilization.
- [ ] Collect and review Linux CI benchmark summaries for at least 10 successful main-branch runs before tightening thresholds.
- [ ] Remove temporary Sonar CPD parser exclusions by extracting shared cron/config parser package used by both `krontab` and `krond`.

## Milestone 4 (MVP Freeze and Alpha Readiness)

- [x] Create a concrete freeze checklist document for `v0.1.0-alpha.1`.
- [ ] Publish `krontab` release binaries for Linux, macOS, and Windows (with checksums).
- [ ] Define pre-1.0 CLI/API stability policy and publish it in docs.
- [x] Prepare changelog section for the first alpha release.
- [ ] Verify "new contributor in under 5 minutes" path from `SETUP.md` and `USAGE.md`.
- [ ] Prepare post-MVP backlog handoff for `daemon/` and `operator/` work.

## Next Stage (Cron Drop-in Replacement)

- [x] Define cron drop-in compatibility profile and boundaries (`docs/CRON-DROPIN.md`).
- [x] Implement initial `krond start` runtime slice (config parse, deterministic scheduling, execution, atomic state).
- [x] Build `krond` cron-source adapters for `/etc/crontab` and `/etc/cron.d/*`.
- [x] Add cron compatibility corpus tests for Tier 1 capabilities.
- [x] Publish migration guide from cron files to Kron runtime execution model.

## Next Release Scope (krond Alpha)

Release blockers (`must`):

- [ ] Freeze compatibility contract across `docs/CRON-DROPIN.md`, `docs/COMPAT.md`, and `docs/CRON-MIGRATION.md`.
- [ ] Resolve docs/spec mismatch for `concurrency=replace` (implement or explicitly defer in all runtime-facing docs).
- [ ] Implement exported user crontab adapter support (or remove from near-term roadmap/scope commitments).
- [ ] Add structured runtime event logging for `executed|skipped|missed|unschedulable|error`.
- [ ] Add integration test for restart idempotency (no duplicate execution after daemon restart).
- [ ] Add integration test for crash/state recovery behavior with invalid or stale state files.
- [ ] Decide and lock release artifact scope for daemon (`krond` binaries + checksums, or explicit `krontab`-only policy).
- [ ] Keep `./scripts/ci.sh` and strict docs build (`sphinx-build -n -W`) green on release branch head.

Quality goals (`should`):

- [ ] Promote benchmark guard in CI from non-blocking to blocking after baseline criteria are met.
- [ ] Publish pre-1.0 stability policy for CLI/API compatibility expectations.
- [ ] Validate and document the "new contributor in under 5 minutes" flow from `SETUP.md` and `USAGE.md`.

Explicitly deferred for this release:

- [ ] `@reboot` semantics
- [ ] `run-parts` compatibility
- [ ] `MAILTO` delivery parity
- [ ] 6-field cron syntax (seconds)
- [ ] implementation-specific bug-for-bug cron parity
