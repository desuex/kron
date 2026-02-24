#!/usr/bin/env bash
set -euo pipefail

fmt_out="$(gofmt -l core cmd)"
if [[ -n "${fmt_out}" ]]; then
  echo "gofmt check failed for:"
  echo "${fmt_out}"
  exit 1
fi

go vet ./core/... ./cmd/krontab/...
go test ./core/... ./cmd/krontab/...
