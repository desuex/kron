#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
usage: ./scripts/bench_check.sh [--report-dir <dir>]

Runs selected daemon benchmarks and validates median ns/op thresholds.
When --report-dir is provided, writes:
  - bench_raw.txt
  - summary.md
  - summary.json
EOF
}

report_dir=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --report-dir)
      if [[ $# -lt 2 ]]; then
        echo "missing value for --report-dir" >&2
        usage >&2
        exit 2
      fi
      report_dir="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

# Initial conservative thresholds for CI regression checks.
# Values are medians across 3 runs, measured in ns/op.
MAX_M1000_NODUE_NS="${MAX_M1000_NODUE_NS:-3000000}"
MAX_M1000_DUE_NS="${MAX_M1000_DUE_NS:-8000000}"
MAX_STATE_SAVE_NS="${MAX_STATE_SAVE_NS:-12000000}"

tmp_file="$(mktemp)"
trap 'rm -f "${tmp_file}"' EXIT

go test ./daemon/pkg/daemon -run '^$' \
  -bench 'BenchmarkRuntimeStepNoDue/M1000_NoDue|BenchmarkRuntimeStepDue/M1000_Due|BenchmarkFileStateStoreSave' \
  -benchmem \
  -count 3 | tee "${tmp_file}"

if [[ -n "${report_dir}" ]]; then
  mkdir -p "${report_dir}"
  cp "${tmp_file}" "${report_dir}/bench_raw.txt"
fi

metric_values() {
  local pattern="$1"
  awk -v p="$pattern" '$1 ~ p { print $3 }' "${tmp_file}"
}

median() {
  sort -n | awk '
    { a[++n] = $1 }
    END {
      if (n == 0) exit 2
      if (n % 2 == 1) { print a[(n + 1) / 2]; exit 0 }
      print int((a[n/2] + a[n/2 + 1]) / 2)
    }'
}

check_threshold() {
  local label="$1"
  local observed="$2"
  local max="$3"
  if (( observed > max )); then
    echo "bench gate failed: ${label} median ${observed} ns/op > ${max} ns/op"
    return 1
  fi
  echo "bench gate passed: ${label} median ${observed} ns/op <= ${max} ns/op"
}

m1000_nodue="$(metric_values '^BenchmarkRuntimeStepNoDue/M1000_NoDue-' | median)"
m1000_due="$(metric_values '^BenchmarkRuntimeStepDue/M1000_Due-' | median)"
state_save="$(metric_values '^BenchmarkFileStateStoreSave-' | median)"

echo "bench medians: M1000_NoDue=${m1000_nodue}ns M1000_Due=${m1000_due}ns StateSave=${state_save}ns"

pass_nodue=true
pass_due=true
pass_save=true

if ! check_threshold "RuntimeStepNoDue/M1000" "${m1000_nodue}" "${MAX_M1000_NODUE_NS}"; then
  pass_nodue=false
fi
if ! check_threshold "RuntimeStepDue/M1000" "${m1000_due}" "${MAX_M1000_DUE_NS}"; then
  pass_due=false
fi
if ! check_threshold "FileStateStoreSave" "${state_save}" "${MAX_STATE_SAVE_NS}"; then
  pass_save=false
fi

overall="pass"
if [[ "${pass_nodue}" != "true" || "${pass_due}" != "true" || "${pass_save}" != "true" ]]; then
  overall="fail"
fi
status_nodue="FAIL"
status_due="FAIL"
status_save="FAIL"
if [[ "${pass_nodue}" == "true" ]]; then
  status_nodue="PASS"
fi
if [[ "${pass_due}" == "true" ]]; then
  status_due="PASS"
fi
if [[ "${pass_save}" == "true" ]]; then
  status_save="PASS"
fi
overall_upper="$(printf '%s' "${overall}" | tr '[:lower:]' '[:upper:]')"

if [[ -n "${report_dir}" ]]; then
  cat > "${report_dir}/summary.md" <<EOF
### Daemon Benchmark Guard

| Metric | Median (ns/op) | Threshold (ns/op) | Status |
|---|---:|---:|---|
| RuntimeStepNoDue/M1000 | ${m1000_nodue} | ${MAX_M1000_NODUE_NS} | ${status_nodue} |
| RuntimeStepDue/M1000 | ${m1000_due} | ${MAX_M1000_DUE_NS} | ${status_due} |
| FileStateStoreSave | ${state_save} | ${MAX_STATE_SAVE_NS} | ${status_save} |

Overall: **${overall_upper}**
EOF

  cat > "${report_dir}/summary.json" <<EOF
{
  "overall": "${overall}",
  "benchmarks": [
    {
      "name": "RuntimeStepNoDue/M1000",
      "median_ns_op": ${m1000_nodue},
      "threshold_ns_op": ${MAX_M1000_NODUE_NS},
      "pass": ${pass_nodue}
    },
    {
      "name": "RuntimeStepDue/M1000",
      "median_ns_op": ${m1000_due},
      "threshold_ns_op": ${MAX_M1000_DUE_NS},
      "pass": ${pass_due}
    },
    {
      "name": "FileStateStoreSave",
      "median_ns_op": ${state_save},
      "threshold_ns_op": ${MAX_STATE_SAVE_NS},
      "pass": ${pass_save}
    }
  ]
}
EOF
fi

if [[ "${overall}" != "pass" ]]; then
  exit 1
fi
