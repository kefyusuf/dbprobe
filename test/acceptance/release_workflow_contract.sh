#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
workflow="$repo_root/.github/workflows/release.yml"

fail() {
  printf 'release workflow contract failed: %s\n' "$1" >&2
  exit 1
}

[[ -f "$workflow" ]] || fail 'release.yml is missing'

require_literal() {
  local value="$1"
  grep -Fq -- "$value" "$workflow" || fail "missing: $value"
}

require_literal "tags:"
require_literal "'v*'"
require_literal "contents: write"
require_literal "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1"
require_literal "actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e"
require_literal "go-version: '1.25.x'"
require_literal "CGO_ENABLED=0"
require_literal "GOOS=linux GOARCH=amd64"
require_literal "GOOS=windows GOARCH=amd64"
require_literal "GOOS=darwin GOARCH=amd64"
require_literal "GOOS=darwin GOARCH=arm64"
require_literal "sha256sum"
require_literal 'gh release create "$GITHUB_REF_NAME"'
require_literal "--verify-tag"

if grep -Eq '^[[:space:]]*(workflow_dispatch|pull_request):' "$workflow"; then
  fail 'release workflow must not be manually or PR triggered'
fi
if grep -Eq '(^|[[:space:]])git[[:space:]]+(tag|push)([[:space:]]|$)' "$workflow"; then
  fail 'release workflow must not create or push tags'
fi
