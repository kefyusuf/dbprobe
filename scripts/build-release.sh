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

if [[ ! "$tag" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
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
mkdir -p "$out_dir"
out_dir="$(cd "$out_dir" && pwd -P)"
stage_root="$(mktemp -d)"
trap 'rm -rf "$stage_root"' EXIT

for legal_file in LICENSE NOTICE THIRD_PARTY_NOTICES.md; do
  if [[ ! -s "$repo_root/$legal_file" ]]; then
    printf 'release requires non-empty %s\n' "$legal_file" >&2
    exit 1
  fi
done

legal_root="$stage_root/.legal"
module_manifest="$stage_root/modules.tsv"
mkdir -p "$legal_root/THIRD_PARTY_LICENSES"
cp "$repo_root/LICENSE" "$repo_root/NOTICE" "$repo_root/THIRD_PARTY_NOTICES.md" "$legal_root/"

(
  cd "$repo_root"
  go mod download all
  go list -m -f '{{if not .Main}}{{.Path}}{{"\t"}}{{.Version}}{{"\t"}}{{.Dir}}{{end}}' all
) > "$module_manifest"

cat >> "$legal_root/THIRD_PARTY_NOTICES.md" <<'EOF'

## Revision-resolved Go module inventory

This section is generated for this release from the resolved `go list -m all`
graph used by the packager. The upstream legal files bundled under
`THIRD_PARTY_LICENSES/` are authoritative.

| Module | Version |
|---|---|
EOF

module_legal_key() {
  local module_path="$1"
  local module_version="$2"
  printf '%s\0%s' "$module_path" "$module_version" | od -An -v -tx1 | tr -d ' \n'
}

while IFS=$'\t' read -r module_path module_version module_dir; do
  [[ -n "$module_path" ]] || continue
  if [[ -z "$module_version" || -z "$module_dir" || ! -d "$module_dir" ]]; then
    printf 'cannot resolve legal material for module %s %s\n' "$module_path" "$module_version" >&2
    exit 1
  fi

  printf '| `%s` | `%s` |\n' "$module_path" "$module_version" \
    >> "$legal_root/THIRD_PARTY_NOTICES.md"

  safe_module="hex-$(module_legal_key "$module_path" "$module_version")"
  module_legal_dir="$legal_root/THIRD_PARTY_LICENSES/$safe_module"
  mkdir -p "$module_legal_dir"
  found=0

  while IFS= read -r -d '' upstream_legal_file; do
    cp "$upstream_legal_file" "$module_legal_dir/$(basename "$upstream_legal_file")"
    found=1
  done < <(
    find "$module_dir" -maxdepth 1 -type f \
      \( -iname 'LICENSE' -o -iname 'LICENSE.*' \
         -o -iname 'COPYING' -o -iname 'COPYING.*' \
         -o -iname 'NOTICE' -o -iname 'NOTICE.*' \
         -o -iname 'COPYRIGHT' -o -iname 'COPYRIGHT.*' \) \
      -print0
  )

  if [[ "$found" -ne 1 ]]; then
    printf 'module %s %s has no root license/notice material\n' "$module_path" "$module_version" >&2
    exit 1
  fi
done < "$module_manifest"

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

  cp -R "$legal_root/." "$stage/"

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
