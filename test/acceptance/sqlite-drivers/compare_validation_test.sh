#!/usr/bin/env bash
set -euo pipefail

out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT

if RUNS=00 SNAPSHOTS=2 OUT="$out/comparison" bash ./compare.sh >/dev/null 2>&1; then
  echo "compare.sh accepted RUNS=00" >&2
  exit 1
fi
