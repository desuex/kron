# TODO

Current working list after Milestone 2 completion.

## Milestone 3 (CLI MVP)

- [ ] Finish remaining syntax parity gaps between `cmd/krontab` and `docs/SYNTAX.md`.
- [ ] Add snapshot-style output tests for `krontab explain` text and JSON modes.
- [ ] Expand integration tests for modifier combinations (`@seed`, `@tz`, `@only`, `@avoid`, `@dist` params).
- [ ] Reconcile `docs/CLI-SPEC.md` examples with actual implemented flags and output fields.

## Documentation and Quality

- [ ] Keep Read the Docs build warning-free under `sphinx-build -n -W`.
- [ ] Add a short "implemented vs planned" matrix in `README.md`.
- [ ] Review spec docs for any stale examples that still imply unimplemented features in MVP commands.

## Next Milestone Preparation

- [ ] Define a concrete Milestone 4 freeze checklist and release criteria (`v0.1.0-alpha.1`).
- [ ] Decide the exact API stability statement for pre-1.0 CLI output.
- [ ] Prepare daemon/operator kickoff backlog after CLI freeze.
