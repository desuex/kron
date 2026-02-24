# SETUP

This guide covers local setup for the current Kron MVP implementation.

## Prerequisites

- Git
- Go 1.22+ (workspace-aware `go` command)

## Clone the Repository

```bash
git clone https://github.com/desuex/kron.git
cd kron
```

## Verify the Toolchain

```bash
go version
go env GOWORK
```

`GOWORK` should resolve to the repository `go.work` file.

## Run Quality Gates

```bash
./scripts/ci.sh
```

This runs formatting checks, `go vet`, tests, and coverage threshold checks.

## Build the MVP CLI

```bash
mkdir -p bin
go build -o bin/krontab ./cmd/krontab
./bin/krontab --help
```

## Current MVP Scope

Implemented commands:

- `krontab lint`
- `krontab explain`
- `krontab next`

For syntax and behavior details, see:

- [SYNTAX.md](SYNTAX.md)
- [KRONTAB.md](KRONTAB.md)
- [CLI-SPEC.md](CLI-SPEC.md)
