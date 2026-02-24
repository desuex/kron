#!/usr/bin/env bash
set -euo pipefail

fmt_out="$(gofmt -l core cmd)"
if [[ -n "${fmt_out}" ]]; then
  echo "gofmt check failed for:"
  echo "${fmt_out}"
  exit 1
fi

go vet ./core/... ./daemon/... ./operator/... ./cmd/krontab/...
go test ./core/... ./daemon/... ./operator/... ./cmd/krontab/...
./scripts/coverage.sh --threshold 90
