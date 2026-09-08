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

tar -C "$extract_dir" -xzf "$out_dir/dbprobe_${version}_linux_amd64.tar.gz"
actual="$("$extract_dir/dbprobe_${version}_linux_amd64/dbprobe" --version)"
expected="dbprobe version ${tag} (commit ${commit}, built ${build_date})"

if [[ "$actual" != "$expected" ]]; then
  printf 'packaged version mismatch\nexpected: %s\nactual:   %s\n' "$expected" "$actual" >&2
  exit 1
fi
