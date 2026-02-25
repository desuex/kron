# RELEASE.md

This document defines the MVP freeze and release process for `krontab`, plus the tracked scope checklist for the next `krond` alpha release.

## Target

First pre-release tag: `v0.1.0-alpha.1`

## Freeze Checklist

- [ ] Working tree is clean on `main`.
- [ ] `./scripts/ci.sh` passes locally.
- [ ] `python3 -m sphinx -n -W -b html docs docs/_build/html` passes locally.
- [ ] `README.md`, `MVP_PLAN.md`, `ROADMAP.md`, and `TODO.md` reflect current milestone state.
- [ ] `CHANGELOG.md` has a prepared section for `v0.1.0-alpha.1`.
- [ ] Release workflow file is up to date and green on the latest `main`.

## Tagging

Create an annotated tag from the release commit:

```bash
git checkout main
git pull --ff-only
git tag -a v0.1.0-alpha.1 -m "v0.1.0-alpha.1"
git push origin v0.1.0-alpha.1
```

## CI Release Outputs

Tag push triggers `.github/workflows/release.yml`.

Expected artifacts:

- `krontab_0.1.0-alpha.1_linux_amd64.tar.gz`
- `krontab_0.1.0-alpha.1_darwin_amd64.tar.gz`
- `krontab_0.1.0-alpha.1_windows_amd64.zip`
- `SHA256SUMS`

## Verification

After the workflow completes:

- Verify GitHub Release exists for tag `v0.1.0-alpha.1`.
- Confirm all three platform archives and `SHA256SUMS` are attached.
- Verify release is marked as pre-release (`alpha` tags).
- Download one artifact and verify checksum against `SHA256SUMS`.

## Next Release Scope (krond Alpha)

Use this checklist to decide release readiness for the next daemon-oriented alpha.

Release blockers (`must`):

- [ ] Freeze `krond` compatibility contract across `docs/CRON-DROPIN.md`, `docs/COMPAT.md`, and `docs/CRON-MIGRATION.md`.
- [ ] Resolve policy mismatch in docs/specs: runtime currently supports `concurrency=allow|forbid`; `replace` stays deferred until implemented.
- [ ] Implement exported user-crontab input adapter (or explicitly remove it from near-term roadmap/scope docs).
- [ ] Add structured runtime events for key outcomes: `executed`, `skipped`, `missed`, `unschedulable`, and execution error.
- [ ] Add restart idempotency integration coverage (no duplicate execution across restart boundary).
- [ ] Add crash/state integrity integration coverage (invalid/corrupt state behavior and recovery path).
- [ ] Decide release artifact scope for daemon and codify it in workflow: publish `krond` binaries (Linux/macOS/Windows + checksums) or explicitly keep release `krontab`-only.
- [ ] Confirm `./scripts/ci.sh` and strict docs build (`sphinx -n -W`) are green on release commit.

Quality goals (`should`):

- [ ] Promote benchmark guard from non-blocking to blocking after baseline stability criteria are met.
- [ ] Publish pre-1.0 stability policy for CLI/API behavior.
- [ ] Re-run and document "new contributor in under 5 minutes" validation from `SETUP.md` + `USAGE.md`.

Explicitly deferred for this release:

- `@reboot` semantics
- `run-parts` compatibility
- `MAILTO` delivery parity
- 6-field cron syntax (seconds)
- bug-for-bug parity with all cron implementations
