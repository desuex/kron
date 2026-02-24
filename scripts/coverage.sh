#!/usr/bin/env bash
set -euo pipefail

threshold=""
if [[ "${1:-}" == "--threshold" ]]; then
  if [[ $# -lt 2 ]]; then
    echo "usage: $0 [--threshold <percent>]" >&2
    exit 2
  fi
  threshold="$2"
  shift 2
fi

if [[ $# -ne 0 ]]; then
  echo "usage: $0 [--threshold <percent>]" >&2
  exit 2
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

core_profile="${tmp_dir}/core.cover.out"
cmd_profile="${tmp_dir}/cmd.cover.out"
combined_profile="${tmp_dir}/combined.cover.out"

go test ./core/... -coverprofile="${core_profile}" -covermode=atomic
go test ./cmd/krontab/... -coverprofile="${cmd_profile}" -covermode=atomic

core_total="$(go tool cover -func="${core_profile}" | awk '/^total:/{print $3}')"
cmd_total="$(go tool cover -func="${cmd_profile}" | awk '/^total:/{print $3}')"

{
  echo "mode: atomic"
  tail -n +2 "${core_profile}"
  tail -n +2 "${cmd_profile}"
} > "${combined_profile}"

combined_total="$(go tool cover -func="${combined_profile}" | awk '/^total:/{print $3}')"

echo "core coverage: ${core_total}"
echo "cmd/krontab coverage: ${cmd_total}"
echo "combined coverage: ${combined_total}"

if [[ -n "${threshold}" ]]; then
  combined_num="${combined_total%\%}"
  if awk -v cov="${combined_num}" -v min="${threshold}" 'BEGIN { exit !(cov + 0 < min + 0) }'; then
    echo "coverage threshold failed: ${combined_total} < ${threshold}%"
    exit 1
  fi
  echo "coverage threshold passed: ${combined_total} >= ${threshold}%"
fi
