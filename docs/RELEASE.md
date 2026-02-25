# RELEASE.md

This document defines the MVP freeze and release process for `krontab`.

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
