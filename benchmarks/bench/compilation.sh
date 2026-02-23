#!/bin/bash
# Compilation benchmark: tsgonest build vs tsc
#
# Measures cold-start compilation time on the realworld fixture
# (6 controllers, 12 source files, ~30 DTOs).
#
# Usage: bash benchmarks/bench/compilation.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TSGONEST="$ROOT/tsgonest"
FIXTURE="$ROOT/testdata/realworld"
TSCONFIG="$FIXTURE/tsconfig.json"
CONFIG="$FIXTURE/tsgonest.config.json"
RUNS=5

# Cross-platform millisecond timer (macOS date doesn't support %3N)
millis() {
  python3 -c "import time; print(int(time.time()*1000))"
}

echo "=== Compilation Benchmark ==="
echo ""
echo "Node.js $(node --version)"
echo "Fixture: testdata/realworld (6 controllers, 12 source files, ~30 DTOs)"
echo "Runs: $RUNS (cold start, --clean each time)"
echo ""

# Verify files exist
if [ ! -f "$TSGONEST" ]; then
  echo "error: tsgonest binary not found at $TSGONEST"
  echo "hint: run 'go build -o tsgonest ./cmd/tsgonest' first"
  exit 1
fi

if [ ! -f "$TSCONFIG" ]; then
  echo "error: tsconfig not found at $TSCONFIG"
  exit 1
fi

# ── tsgonest build (full pipeline: compile + companions + OpenAPI) ──

echo "--- tsgonest build (compile + companions + OpenAPI) ---"
tsgonest_sum=0
for i in $(seq 1 $RUNS); do
  start=$(millis)
  "$TSGONEST" build --project "$TSCONFIG" --config "$CONFIG" --clean 2>/dev/null
  end=$(millis)
  elapsed=$((end - start))
  tsgonest_sum=$((tsgonest_sum + elapsed))
  echo "  run $i: ${elapsed}ms"
done
tsgonest_avg=$((tsgonest_sum / RUNS))
echo "  average: ${tsgonest_avg}ms"
echo ""

# ── tsc (TypeScript compiler, no companions/OpenAPI) ────────────────

echo "--- tsc (TypeScript compiler only) ---"
TSC="$ROOT/node_modules/.bin/tsc"
if [ ! -f "$TSC" ]; then
  TSC="$(which tsc 2>/dev/null || true)"
fi
if [ -z "$TSC" ] || [ ! -f "$TSC" ]; then
  echo "  tsc not found, skipping"
  echo ""
else
  tsc_sum=0
  for i in $(seq 1 $RUNS); do
    # Clean dist
    rm -rf "$FIXTURE/dist"
    start=$(millis)
    "$TSC" --project "$TSCONFIG" 2>/dev/null || true
    end=$(millis)
    elapsed=$((end - start))
    tsc_sum=$((tsc_sum + elapsed))
    echo "  run $i: ${elapsed}ms"
  done
  tsc_avg=$((tsc_sum / RUNS))
  echo "  average: ${tsc_avg}ms"
  echo ""

  # ── Comparison ────────────────────────────────────────────────────
  echo "--- Summary ---"
  echo "  tsgonest build (full): ${tsgonest_avg}ms avg"
  echo "  tsc (compile only):    ${tsc_avg}ms avg"
  if [ "$tsgonest_avg" -gt 0 ]; then
    ratio=$(python3 -c "print(f'{$tsc_avg / $tsgonest_avg:.1f}')")
    echo "  tsgonest is ${ratio}x faster than tsc (while also generating companions + OpenAPI)"
  fi
fi
