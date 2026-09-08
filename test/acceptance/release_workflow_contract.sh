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
require_literal "contents: read"
require_literal "contents: write"
require_literal "build:"
require_literal "publish:"
require_literal "needs: build"
require_literal "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1"
require_literal "actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e"
require_literal "actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a"
require_literal "actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c"
require_literal "go-version: '1.25.x'"
require_literal 'CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch"'
require_literal "build_archive linux amd64 '' tar"
require_literal "build_archive windows amd64 '.exe' zip"
require_literal "build_archive darwin amd64 '' tar"
require_literal "build_archive darwin arm64 '' tar"
require_literal "sha256sum"
require_literal 'gh release create "$GITHUB_REF_NAME"'
require_literal "--verify-tag"
require_literal 'git merge-base --is-ancestor "$GITHUB_SHA" origin/main'

if grep -Eq '^[[:space:]]*(workflow_dispatch|pull_request):' "$workflow"; then
  fail 'release workflow must not be manually or PR triggered'
fi
if grep -Eq '(^|[[:space:]])git[[:space:]]+(tag|push)([[:space:]]|$)' "$workflow"; then
  fail 'release workflow must not create or push tags'
fi
