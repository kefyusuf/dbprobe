#!/usr/bin/env bash
set -euo pipefail

runs=${RUNS:-7}
snapshots=${SNAPSHOTS:-250}
out=${OUT:-/tmp/dbprobe-sqlite-driver-comparison}

if ! [[ "$runs" =~ ^[1-9][0-9]*$ ]] ||
   ! [[ "$snapshots" =~ ^([2-9]|[1-9][0-9]+)$ ]]; then
  echo "RUNS and SNAPSHOTS must be canonical positive integers; SNAPSHOTS must be at least 2" >&2
  exit 2
fi

rm -rf "$out"
mkdir -p "$out"

CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$out/modernc" ./cmd/modernc
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$out/ncruces" ./cmd/ncruces

modernc_size=$(wc -c < "$out/modernc" | tr -d ' ')
ncruces_size=$(wc -c < "$out/ncruces" | tr -d ' ')
printf '{"kind":"binary","driver":"modernc.org/sqlite","bytes":%s}\n' "$modernc_size"
printf '{"kind":"binary","driver":"github.com/ncruces/go-sqlite3","bytes":%s}\n' "$ncruces_size"

export GOMAXPROCS=${GOMAXPROCS:-2}
export DBPROBE_SQLITE_COMPARE_SNAPSHOTS=$snapshots

"$out/modernc" >/dev/null
"$out/ncruces" >/dev/null

run_one() {
  local run=$1
  local driver=$2
  local binary=$3
  local started_ns finished_ns process_elapsed_ns result
  started_ns=$(date +%s%N)
  result=$("$binary")
  finished_ns=$(date +%s%N)
  process_elapsed_ns=$((finished_ns - started_ns))
  printf '{"kind":"run","run":%s,"candidate":"%s","process_elapsed_ns":%s,"result":%s}\n' \
    "$run" "$driver" "$process_elapsed_ns" "$result"
}

for ((run=1; run<=runs; run++)); do
  if (( run % 2 == 1 )); then
    run_one "$run" "modernc.org/sqlite" "$out/modernc"
    run_one "$run" "github.com/ncruces/go-sqlite3" "$out/ncruces"
  else
    run_one "$run" "github.com/ncruces/go-sqlite3" "$out/ncruces"
    run_one "$run" "modernc.org/sqlite" "$out/modernc"
  fi
done
