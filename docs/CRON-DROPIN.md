# CRON-DROPIN.md

## Purpose

Define what "cron drop-in replacement" means for Kron's next stage and set a realistic compatibility boundary.

Goal: high compatibility for common cron workloads while preserving Kron's deterministic core semantics.

## Coverage Objective

Target practical compatibility for common Linux cron workloads:

- expected coverage band: `~70-80%` of real-world cron usage
- full parity with every cron implementation variant is explicitly out of scope for this stage

## Compatibility Tiers

- `Tier 1 (Supported)`: expected to work and covered by integration tests
- `Tier 2 (Partial)`: supported with documented behavior differences
- `Tier 3 (Deferred)`: not implemented in this stage

## Planned Compatibility Matrix

| Capability | Stage target | Tier | Notes |
|---|---|---|---|
| 5-field cron syntax (`*`, ranges, lists, steps, names) | yes | Tier 1 | Matches current parser baseline |
| DOM/DOW OR matching semantics | yes | Tier 1 | Keep Vixie-compatible semantics |
| `/etc/crontab` and `/etc/cron.d/*` style entries | yes | Tier 1 | Includes system crontab user field |
| Import/export of user crontab entries | yes | Tier 2 | Migration tooling required |
| Environment assignment lines (`PATH=...`) | yes | Tier 1 | Parsed and applied per job |
| Command execution through shell-compatible mode | yes | Tier 1 | Required for common cron command patterns |
| Explicit user execution (system crontab user column) | yes | Tier 2 | Host and permission model dependent |
| Structured logs and deterministic explain metadata | yes | Tier 1 | Kron-native enhancement |
| `@daily`/`@weekly`/`@monthly` shortcuts | yes | Tier 2 | Expand to canonical cron expressions |
| `@reboot` semantics | no | Tier 3 | Deferred until daemon boot lifecycle is finalized |
| 6-field (seconds) cron syntax | no | Tier 3 | Not part of current syntax contract |
| `MAILTO` and mail delivery behavior | no | Tier 3 | Deferred; logging is primary output |
| `run-parts` directory conventions (`/etc/cron.hourly`) | no | Tier 3 | Deferred and platform-specific |
| anacron-style unlimited catch-up behavior | no | Tier 3 | Conflicts with bounded execution design |

## Non-Goals for This Stage

- Bug-for-bug parity with every cron implementation (`cronie`, `dcron`, BusyBox, distro patches)
- Full legacy ecosystem parity (`MAILTO`, PAM-specific hooks, `run-parts` behaviors)
- Unbounded missed-run replay semantics

## Acceptance Criteria

- Tier 1 capabilities are implemented and covered by integration tests.
- Tier 2 capabilities are either implemented with explicit docs or marked as deferred with rationale.
- Compatibility corpus is added under version-controlled tests and runs in CI.
- Migration guide from existing cron files to Kron execution model is published.

## Execution Plan

1. Build cron-source adapters (`/etc/crontab`, `/etc/cron.d`, exported user crontabs).
2. Normalize parsed entries into Kron internal job spec.
3. Implement host execution loop in `krond` with locking, deadline handling, and deterministic decision flow.
4. Add compatibility tests and pessimistic edge-case tests for cron parsing and runtime behavior.
5. Publish compatibility status updates in `docs/COMPAT.md` and release notes.
