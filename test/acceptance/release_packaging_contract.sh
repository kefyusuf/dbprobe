#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
out_dir="$(mktemp -d)"
extract_dir="$(mktemp -d)"
trap 'rm -rf "$out_dir" "$extract_dir"' EXIT

tag='v0.1.0'
commit='0123456789abcdef0123456789abcdef01234567'
build_date='2026-09-08T20:10:00Z'
version="${tag#v}"

cd "$repo_root"
bash ./scripts/build-release.sh "$tag" "$commit" "$build_date" "$out_dir"

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
actual="$($extract_dir/dbprobe_${version}_linux_amd64/dbprobe --version)"
expected="dbprobe version ${tag} (commit ${commit}, built ${build_date})"

if [[ "$actual" != "$expected" ]]; then
  printf 'packaged version mismatch\nexpected: %s\nactual:   %s\n' "$expected" "$actual" >&2
  exit 1
fi
