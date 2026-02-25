# SECURITY.md

## Scope

This document defines the security model, threat model, and hardening requirements for Kron.

It applies to:

* `kron-core`
* `krond`
* `kron-operator`

Security goals:

* Prevent privilege escalation.
* Prevent arbitrary code injection via configuration.
* Preserve integrity of scheduling decisions.
* Preserve integrity of state.
* Minimize required privileges.
* Provide auditable behavior.

---

## Threat Model

Kron assumes:

* The host OS kernel is trusted.
* The Go runtime is trusted.
* The system clock may change but is not maliciously controlled.
* The timezone database is trusted.

Kron does not assume:

* Configuration files are always trusted.
* State files are always intact.
* Environment variables are safe.
* External commands are safe.

Kron must defend against:

* Malicious configuration input.
* Accidental misconfiguration.
* Local file tampering.
* Unprivileged users attempting privilege escalation.
* Injection via shell expansion.
* Duplicate execution via race conditions.
* Supply-chain compromise (release artifacts).

---

## Trust Boundaries

### kron-core

* Pure computation.
* No I/O.
* No privilege boundary.
* No external dependencies at runtime.

Security concerns are limited to deterministic behavior and parameter validation.

---

### krond

Trust boundary between:

* Daemon process
* Executed child processes
* Configuration files
* State files
* Operating system user accounts

`krond` may run as root or as a dedicated service user.

Child processes are untrusted.

---

### kron-operator

Trust boundary between:

* Kubernetes API server
* Controller pod
* User-defined `KronJob` specs
* Created `Job` objects

Controller must not allow privilege escalation via CRD fields.

---

## Principle of Least Privilege

### krond

* Must not require root unless user switching is configured.
* If root, must drop privileges before executing commands.
* Must allow running as non-root service user.
* State directory must be owned by service user.

### kron-operator

* Must request only:

  * Read/write access to `KronJob` resources.
  * Create/list/watch `Job` resources in allowed namespaces.
  * Emit Events.
* Must not require cluster-admin privileges.
* Must support namespace-scoped deployment.

---

## Configuration Security

Configuration files:

* Must be readable only by owner.
* Must not be world-writable.
* Must reject invalid syntax.
* Must reject unknown directives.

`krontab` must validate syntax before writing configuration.

---

## Command Execution Security

### No Implicit Shell

Commands must not be executed via shell by default.

Default execution:

```text
execve(binary, args, env)
```

Shell wrapping must require explicit configuration.

No automatic expansion of:

* `*`
* `$VAR`
* `~`
* Backticks
* Pipes

---

### Argument Handling

Arguments must be passed as structured array.

String splitting must not occur implicitly.

If shell mode enabled, behavior must be documented and explicit.

---

## Privilege Dropping

If running as root and job specifies user:

1. Validate user exists.
2. Resolve UID/GID.
3. Call `setgid()` before `setuid()`.
4. Clear supplementary groups unless explicitly configured.
5. Verify privilege drop succeeded.

If privilege drop fails:

* Abort execution.
* Log error.
* Do not execute as root unintentionally.

---

## Environment Isolation

Child process environment:

* Inherits only explicitly allowed variables.
* May optionally inherit full environment if configured.
* Must allow full override.

Sensitive environment variables must not be injected implicitly.

---

## State File Security

State directory:

```text
0700
```

State files:

```text
0600
```

State file contents must not include:

* Secrets
* Full command lines with sensitive arguments (unless explicitly allowed)

State integrity must be protected via atomic writes.

If file permissions are weaker than required:

* Daemon must refuse to start.

---

## Symlink and Path Safety

Before writing state file:

* Ensure file path is not symlink.
* Use `O_NOFOLLOW` where available.
* Validate directory ownership.

Before reading configuration:

* Ensure file path is not symlink unless explicitly allowed.

---

## PID Validation

When recovering active execution:

* Verify PID exists.
* Optionally verify command line matches expected binary.
* Avoid trusting stale PID blindly.

---

## Denial of Service Mitigation

Kron must protect against:

* Excessively large configuration files.
* Excessive number of jobs.
* Excessive sampling attempts.

Limits must exist for:

* Maximum number of jobs.
* Maximum window duration.
* Maximum constraint clauses.
* Maximum sampling attempts.

Violations must cause validation error.

---

## Resource Limits

`krond` may enforce:

* Maximum concurrent executions.
* Maximum child process runtime (timeout).
* Maximum open files.

Must avoid unbounded memory allocation.

---

## Timezone and Locale Safety

Timezone parsing must use IANA database only.

Locale must not affect scheduling semantics.

String comparisons must be byte-stable.

---

## Supply Chain Security

Release artifacts must:

* Be reproducible.
* Be signed.
* Provide checksums.

Container images must:

* Be minimal.
* Avoid unnecessary binaries.
* Avoid shell unless required.
* Run as non-root by default (operator).

Dependencies must be pinned via go modules.

---

## Logging Safety

Logs must not:

* Leak secrets.
* Log environment variables by default.
* Log full command lines unless explicitly allowed.

Sensitive fields must be redacted if marked confidential.

---

## Kubernetes-Specific Security

`kron-operator` must:

* Not allow arbitrary Pod specs beyond what `Job` allows.
* Respect namespace boundaries.
* Not escalate privileges via service accounts.
* Use leader election securely.

Owner references must prevent orphaned Jobs.

RBAC examples must follow least privilege.

---

## Upgrade and Migration Security

State migrations must:

* Validate input schema.
* Preserve idempotency.
* Reject unknown or corrupted state.

No silent fallback to empty state.

---

## Auditability

Kron must log:

* Privilege drops.
* Command execution start.
* Command execution termination.
* State corruption detection.
* Configuration reloads.
* Fatal exits.

Logs must allow post-incident analysis.

---

## Cryptographic Requirements

Seed hashing must use a stable cryptographic hash.

Hash algorithm must not change within major version.

Seed material must not be truncated.

No insecure random number generator allowed.

---

## Non-Goals

Kron does not:

* Sandbox executed commands.
* Prevent malicious commands from performing harmful actions.
* Encrypt state files.
* Protect against a fully compromised host.

Kron ensures scheduling integrity, not workload security.

---

## Security Invariants

Kron guarantees:

1. No command is executed implicitly through shell.
2. No privilege escalation beyond configuration.
3. No duplicate execution due to race or crash.
4. No execution with invalid configuration.
5. State integrity is required for execution.
6. Determinism cannot be influenced by environment variables.
7. Sensitive data is not logged by default.

Correctness, determinism, and least privilege are mandatory.
