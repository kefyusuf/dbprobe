#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
bin="$(mktemp)"
trap 'rm -f "$bin"' EXIT

version='v0.1.0-test'
commit='0123456789abcdef0123456789abcdef01234567'
build_date='2026-09-08T19:45:00Z'

cd "$repo_root"
CGO_ENABLED=0 go build \
  -ldflags "-X main.version=${version} -X main.commit=${commit} -X main.buildDate=${build_date}" \
  -o "$bin" \
  ./cmd/dbprobe

expected="dbprobe version ${version} (commit ${commit}, built ${build_date})"
actual="$($bin --version)"

if [[ "$actual" != "$expected" ]]; then
  printf 'version output mismatch\nexpected: %s\nactual:   %s\n' "$expected" "$actual" >&2
  exit 1
fi
