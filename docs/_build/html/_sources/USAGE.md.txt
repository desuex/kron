# USAGE

This guide shows practical usage of the current `krontab` MVP commands.

Runtime distribution scope in MVP:

- Executed by `explain`/`next`: `uniform`, `skewEarly`, `skewLate`
- Accepted by `lint` but not executed by MVP runtime: `normal`, `exponential`

## 1. Create a Configuration File

Example `example.kron`:

```text
0 0 * * * @tz(UTC) @win(after,2h) @dist(uniform) @seed(stable,salt=backup) name=db-backup command=/usr/bin/backup
*/30 * * * * @tz(America/New_York) @win(after,15m) @dist(skewEarly,shape=2) @only(hours=8-18;dow=MON-FRI) name=reporting command=/usr/local/bin/report
```

## 2. Validate Configuration

```bash
krontab lint --file ./example.kron --format text
krontab lint --file ./example.kron --format json
```

Exit codes:

- `0`: valid
- `1`: invalid configuration
- `2`: command/input/system error

## 3. Explain One Period Decision

```bash
krontab explain db-backup --file ./example.kron --at 2026-03-01T00:00:00Z --format text
krontab explain db-backup --file ./example.kron --at 2026-03-01T00:00:00Z --format json
```

Use this command to review the deterministic output for a specific period.

## 4. List Upcoming Decisions

```bash
krontab next reporting --file ./example.kron --count 5 --at 2026-03-01T00:00:00Z --format text
krontab next reporting --file ./example.kron --count 5 --at 2026-03-01T00:00:00Z --format json
```

This returns the next N computed periods and their chosen times.

## 5. Troubleshooting

- `error: job not found`: verify `name=<job>` exists in the provided file.
- `invalid --at value`: use RFC3339, for example `2026-03-01T00:00:00Z`.
- modifier validation errors: check syntax against [SYNTAX.md](SYNTAX.md).

## Related References

- [KRONTAB.md](KRONTAB.md)
- [SYNTAX.md](SYNTAX.md)
- [CORE-SPEC.md](CORE-SPEC.md)
- [TEST-VECTORS.md](TEST-VECTORS.md)
