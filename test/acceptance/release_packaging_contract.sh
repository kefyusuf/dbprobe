#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
out_dir_rel=".release-test-output-${RANDOM}-${RANDOM}"
out_dir="$repo_root/$out_dir_rel"
extract_parent="$repo_root/.release test-${RANDOM}-${RANDOM}"
extract_dir="$extract_parent/extract"
validation_log="$(mktemp)"
mkdir -p "$extract_dir"
trap 'rm -rf "$out_dir" "$extract_parent"; rm -f "$validation_log"' EXIT

tag='v0.1.0'
commit='0123456789abcdef0123456789abcdef01234567'
build_date='2026-09-08T20:10:00Z'
version="${tag#v}"

cd "$repo_root"

for invalid_tag in v01.2.3 v1.02.3 v1.2.03; do
  : > "$validation_log"
  if bash ./scripts/build-release.sh "$invalid_tag" not-a-valid-commit "$build_date" "$out_dir_rel-invalid" > /dev/null 2> "$validation_log"; then
    printf 'noncanonical release tag unexpectedly succeeded: %s\n' "$invalid_tag" >&2
    exit 1
  fi
  if ! grep -Fq 'release tag must match vMAJOR.MINOR.PATCH' "$validation_log"; then
    printf 'noncanonical release tag was not rejected by tag validation: %s\n' "$invalid_tag" >&2
    cat "$validation_log" >&2
    exit 1
  fi
done

bash ./scripts/build-release.sh "$tag" "$commit" "$build_date" "$out_dir_rel"

expected_files=(
  "dbprobe_${version}_linux_amd64.tar.gz"
  "dbprobe_${version}_windows_amd64.zip"
  "dbprobe_${version}_darwin_amd64.tar.gz"
  "dbprobe_${version}_darwin_arm64.tar.gz"
  "dbprobe_${version}_checksums.txt"
)

for file in "${expected_files[@]}"; do
  [[ -s "$out_dir/$file" ]] || {
    printf 'missing or empty release asset: %s\n' "$file" >&2
    exit 1
  }
done

(
  cd "$out_dir"
  sha256sum -c "dbprobe_${version}_checksums.txt"
)

module_graph="$(go list -m -f '{{if not .Main}}{{.Path}}{{"\t"}}{{.Version}}{{end}}' all)"
archive_specs=(
  "dbprobe_${version}_linux_amd64.tar.gz|dbprobe_${version}_linux_amd64|tar"
  "dbprobe_${version}_windows_amd64.zip|dbprobe_${version}_windows_amd64|zip"
  "dbprobe_${version}_darwin_amd64.tar.gz|dbprobe_${version}_darwin_amd64|tar"
  "dbprobe_${version}_darwin_arm64.tar.gz|dbprobe_${version}_darwin_arm64|tar"
)

for spec in "${archive_specs[@]}"; do
  IFS='|' read -r archive prefix kind <<< "$spec"
  if [[ "$kind" == "zip" ]]; then
    listing="$(unzip -Z1 "$out_dir/$archive")"
  else
    listing="$(tar -tzf "$out_dir/$archive")"
  fi

  for legal_file in LICENSE NOTICE THIRD_PARTY_NOTICES.md; do
    if ! grep -Fxq "$prefix/$legal_file" <<< "$listing"; then
      printf 'release archive %s is missing %s\n' "$archive" "$legal_file" >&2
      exit 1
    fi
  done

  while IFS=$'\t' read -r module_path module_version; do
    [[ -n "$module_path" ]] || continue
    safe_module="${module_path//\//__}@${module_version}"
    if ! grep -Fq "$prefix/THIRD_PARTY_LICENSES/$safe_module/" <<< "$listing"; then
      printf 'release archive %s is missing license material for %s %s\n' \
        "$archive" "$module_path" "$module_version" >&2
      exit 1
    fi
  done <<< "$module_graph"
done

tar -C "$extract_dir" -xzf "$out_dir/dbprobe_${version}_linux_amd64.tar.gz"
prefix="dbprobe_${version}_linux_amd64"
actual="$("$extract_dir/$prefix/dbprobe" --version)"
expected="dbprobe version ${tag} (commit ${commit}, built ${build_date})"

if [[ "$actual" != "$expected" ]]; then
  printf 'packaged version mismatch\nexpected: %s\nactual:   %s\n' "$expected" "$actual" >&2
  exit 1
fi

while IFS=$'\t' read -r module_path module_version; do
  [[ -n "$module_path" ]] || continue
  if ! grep -Fq "| \`$module_path\` | \`$module_version\` |" "$extract_dir/$prefix/THIRD_PARTY_NOTICES.md"; then
    printf 'third-party notice is missing module %s %s\n' "$module_path" "$module_version" >&2
    exit 1
  fi
done <<< "$module_graph"
