#!/usr/bin/env bash
set -euo pipefail

out_dir="${1:-artifacts/bench}"
mkdir -p "${out_dir}"

stamp="$(date -u +%Y%m%dT%H%M%SZ)"
out_file="${out_dir}/daemon_${stamp}.txt"

echo "Running daemon benchmarks..."
echo "Output file: ${out_file}"

go test ./daemon/pkg/daemon -run '^$' \
  -bench 'BenchmarkRuntimeStep|BenchmarkFileStateStore' \
  -benchmem \
  -count 3 | tee "${out_file}"

echo "Benchmark report saved: ${out_file}"
