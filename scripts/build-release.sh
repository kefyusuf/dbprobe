#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 4 ]]; then
  echo 'usage: build-release.sh <tag> <commit> <build-date> <output-dir>' >&2
  exit 2
fi

tag="$1"
commit="$2"
build_date="$3"
out_dir="$4"

if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo 'release tag must match vMAJOR.MINOR.PATCH' >&2
  exit 1
fi
if [[ ! "$commit" =~ ^[0-9a-f]{40}$ ]]; then
  echo 'release commit must be a 40-character lowercase hexadecimal SHA' >&2
  exit 1
fi
if [[ -z "$build_date" ]]; then
  echo 'release build date must not be empty' >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${tag#v}"
stage_root="$(mktemp -d)"
trap 'rm -rf "$stage_root"' EXIT
mkdir -p "$out_dir"

ldflags="-s -w -X main.version=${tag} -X main.commit=${commit} -X main.buildDate=${build_date}"

build_archive() {
  local goos="$1"
  local goarch="$2"
  local extension="$3"
  local archive_kind="$4"
  local name="dbprobe_${version}_${goos}_${goarch}"
  local stage="$stage_root/$name"

  mkdir -p "$stage"
  (
    cd "$repo_root"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build \
      -trimpath \
      -ldflags "$ldflags" \
      -o "$stage/dbprobe${extension}" \
      ./cmd/dbprobe
  )

  if [[ "$goos" == "linux" && "$goarch" == "amd64" ]]; then
    local expected="dbprobe version ${tag} (commit ${commit}, built ${build_date})"
    local actual
    actual="$("$stage/dbprobe" --version)"
    if [[ "$actual" != "$expected" ]]; then
      printf 'release version mismatch\nexpected: %s\nactual:   %s\n' "$expected" "$actual" >&2
      exit 1
    fi
  fi

  if [[ "$archive_kind" == "zip" ]]; then
    (cd "$stage_root" && zip -q -r "$out_dir/${name}.zip" "$name")
  else
    tar -C "$stage_root" -czf "$out_dir/${name}.tar.gz" "$name"
  fi
}

build_archive linux amd64 '' tar
build_archive windows amd64 '.exe' zip
build_archive darwin amd64 '' tar
build_archive darwin arm64 '' tar

(
  cd "$out_dir"
  sha256sum \
    "dbprobe_${version}_linux_amd64.tar.gz" \
    "dbprobe_${version}_windows_amd64.zip" \
    "dbprobe_${version}_darwin_amd64.tar.gz" \
    "dbprobe_${version}_darwin_arm64.tar.gz" \
    > "dbprobe_${version}_checksums.txt"
)
