# daemon

`daemon` hosts the `krond` adapter implementation.

Current state: early usable daemon slice.

Implemented:

- `krond start --config <file|dir> [--source kron|cron]` command (`daemon/cmd/krond`)
- kron config parsing for runtime-supported modifiers (`--source kron`)
- system cron source parsing for `/etc/crontab` and `/etc/cron.d/*`-style entries (`--source cron`)
- user/group-aware execution identity (root-required for switching to different accounts)
- deterministic scheduling decisions via `kron-core`
- async process execution with `@policy(concurrency=allow|forbid)` semantics
- single-instance state-dir lock (`.krond.lock`) to prevent multi-process overlap
- optional shell/env/cwd/timeout command settings
- per-job atomic state persistence (`last handled period`) for restart idempotency

Out of scope in this slice:

- hot reload
- full cron drop-in ecosystem parity (`MAILTO`, `run-parts`, complete host integration, etc.)
