# TODO

Tracking remaining specification and documentation work before implementation begins.

## Specs still pending review

- [ ] ERROR-MODEL.md — review ExecutionError edge cases (fork fails vs exec fails)
- [ ] STATE.md — confirm crash recovery flow with real fsync semantics
- [ ] SECURITY.md — validate privilege drop sequence on Linux vs Darwin
- [ ] COMPAT.md — add concrete DST test cases
- [ ] TEST-VECTORS.md — add vectors for `normal` and `exponential` distributions
- [ ] CRD-SPEC.md — finalize Job naming scheme (`<kronjob-name>-<period-hash>`)
- [ ] CLI-SPEC.md — define `krontab` config file format (referenced but not specified)

## Missing documentation

- [ ] `krontab` file format specification (SYNTAX.md covers modifiers but not the full file format for `krond`)
- [ ] Deployment guide for `kron-operator` (Helm values, RBAC examples)
- [ ] Deployment guide for `krond` (systemd unit, permissions setup)

## Structural

- [ ] Decide whether MANIFESTO.md should remain in `docs/` or move to project root
