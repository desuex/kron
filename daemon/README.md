# daemon

`daemon` hosts the `krond` adapter implementation.

Current state: early usable daemon slice.

Implemented:

- `krond start --config <file|dir>` command (`daemon/cmd/krond`)
- krontab file parsing for runtime-supported modifiers
- deterministic scheduling decisions via `kron-core`
- synchronous process execution with optional shell/env/cwd/timeout
- per-job atomic state persistence (`last handled period`) for restart idempotency

Out of scope in this slice:

- lock file / single-instance enforcement
- hot reload
- full cron drop-in ecosystem parity (`MAILTO`, `run-parts`, etc.)
