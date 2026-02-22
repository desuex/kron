# KRONTAB File Format Specification

## Scope

This document defines the file format for `krond` configuration files.

It specifies:

- File structure
- Job entry format
- Field syntax
- Parsing rules
- Validation rules
- Compatibility guarantees

This document defines file format only.
Scheduling semantics are defined in SYNTAX.md and SPEC.md.

---

## File Location

System directory:

    /etc/krond.d/

Per-user directory:

    ~/.config/krond/

Each file may contain one or more job entries.

Files are processed independently.
There is no guaranteed ordering between files.

---

## Encoding

- UTF-8
- Unix line endings (`\n`)
- No BOM

---

## Comments

Lines beginning with:

    #

are comments and ignored.

Inline comments are not supported.

Example:

    # nightly backup
    0 0 * * * @win(after,2h) name=nightly-backup command=/usr/bin/backup

---

## Empty Lines

Empty lines are ignored.

---

## Job Entry Structure

Each job entry consists of:

1. A schedule expression with optional modifiers
2. One or more key=value fields
3. All on a single logical line

Canonical form:

    <schedule-and-modifiers> <field> <field> ...

Example:

    0 0 * * * @win(after,2h) @dist(uniform) name=db-backup command=/usr/bin/backup

---

## Schedule and Modifiers

The schedule and modifiers must conform exactly to SYNTAX.md.

It must appear first on the line.

Example:

    0 10 * * * @tz(Europe/Paris) @win(around,45m)

Invalid schedule causes configuration error.

---

## Fields

Fields are key=value pairs separated by spaces.

General form:

    key=value

Keys are case-sensitive.

Unknown keys cause validation error.

---

## Required Fields

### name

Stable identifier for the job entry.

    name=db-backup

Must be unique within a file. Used for identity derivation (see Job Identity).

Must be a non-empty string containing only lowercase alphanumeric characters, hyphens, and forward slashes.

---

### command

Defines executable path and arguments.

Example:

    command=/usr/bin/backup

OR with arguments:

    command="/usr/bin/backup --full"

Command parsing rules:

- If unquoted:
  - Entire value until next space is used.
  - No argument splitting performed.
- If quoted:
  - Double quotes required.
  - Escaped quotes supported using \".
  - If `shell=true`, value is passed to `/bin/sh -c`.
  - If `shell=false` or omitted, value is split into binary and arguments.

Shell execution is disabled by default. See `shell` field below.

---

## Optional Fields

### user

User to run command as.

    user=backup

If omitted:
- Run as daemon user.

---

### group

Primary group for execution.

    group=backup

If omitted:
- Use user’s default group.

---

### cwd

Working directory.

    cwd=/var/backups

If omitted:
- Inherit daemon working directory.

---

### env

Environment variable.

May appear multiple times on the same line.

    env=FOO=bar env=MODE=prod

Each `env` entry sets exactly one environment variable.

---

### shell

Enable shell execution.

    shell=true

If `true`, command is passed to `/bin/sh -c`.

If omitted or `false`, command is executed directly via `execve()`.

---

### umask

File creation mask for the child process.

    umask=0027

Octal format required.

If omitted:
- Inherit daemon umask.

---

### timeout

Execution timeout.

Go duration format.

    timeout=10m
    timeout=30s

If omitted:
- No timeout.

---

### stdout

STDOUT handling.

Allowed values:

- `inherit` — write to daemon stdout
- `discard` — suppress output
- `file:<path>` — write to file
- `syslog` — write to system log

Example:

    stdout=file:/var/log/backup.out

Default: `inherit`.

---

### stderr

STDERR handling.

Allowed values:

- `inherit` — write to daemon stderr
- `discard` — suppress output
- `file:<path>` — write to file
- `syslog` — write to system log

Default: `inherit`.

---

### description

Optional human-readable description.

    description="Nightly full backup"

Does not affect scheduling.

---

## Multi-Line Entries

Not supported.

Each job must be fully defined on one line.

---

## Quoting Rules

- Double quotes required for values containing spaces.
- Escape sequence:

    \"  → literal quote
    \\  → literal backslash

No other escape sequences supported.

Example:

    command="/usr/bin/backup --target \"primary cluster\""

---

## Duplicate Fields

Rules:

- `name` may appear only once.
- `command` may appear only once.
- `user` may appear at most once.
- `group` may appear at most once.
- `cwd` may appear at most once.
- `shell` may appear at most once.
- `umask` may appear at most once.
- `timeout` may appear at most once.
- `stdout` may appear at most once.
- `stderr` may appear at most once.
- `description` may appear at most once.
- `env` may appear multiple times.

Duplicate forbidden fields cause validation error.

---

## Job Identity

Identity is derived as:

    <config_path>:<name>

Where `<config_path>` is the absolute path to the configuration file and `<name>` is the value of the `name` field.

The `name` field is required. Each name must be unique within a file.

Identity is used for seed derivation, state persistence, locking, and logging. Changing the name changes the identity, which changes all future scheduling decisions.

Identity must be stable across daemon restarts and configuration reloads.

---

## Validation Rules

Configuration must fail validation if:

- Schedule invalid.
- Required `name` missing.
- Required `command` missing.
- Duplicate `name` within a file.
- Unknown field present.
- Invalid duration format.
- Invalid quoting.
- Duplicate forbidden fields.
- Empty command.
- Relative command path when strict mode enabled.

Validation occurs before daemon starts.

---

## Security Rules

- Files must not be world-writable.
- If group-writable, group must match daemon group.
- Symlinks are not followed unless explicitly allowed.
- File ownership must be verified.

If security validation fails:
- Configuration is rejected.

---

## Examples

### Minimal exact schedule

    0 0 * * * name=backup command=/usr/bin/backup

---

### Load smoothing

    0 0 * * * @win(after,3h) @dist(uniform) name=db-backup command=/usr/bin/backup

---

### Human-like timing

    0 10 * * * @tz(Europe/Paris) @win(around,90m) @dist(skewLate,shape=2.5) name=batch-messages command="/usr/bin/send-messages --batch"

---

### With environment and timeout

    0 2 * * * @win(after,1h) name=cleanup command=/usr/bin/cleanup env=MODE=prod timeout=20m

---

## Compatibility Guarantees

Within a major version:

- Field names will not change.
- Parsing rules will not change.
- Quoting rules will not change.
- Identity derivation will not change.

New optional fields may be added.

Unknown fields must always produce validation error.

---

## Non-Goals

KRONTAB format does not:

- Support interactive editing.
- Support environment blocks.
- Support multi-line commands.
- Support implicit shell execution.
- Support file includes.

It is intentionally minimal and explicit.