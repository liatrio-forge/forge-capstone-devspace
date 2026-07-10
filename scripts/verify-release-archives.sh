#!/usr/bin/env bash
set -euo pipefail

dist=${1:-dist}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
checksums="$dist/checksums.txt"

if [[ ! -f "$checksums" ]]; then
  echo "missing checksums.txt" >&2
  exit 1
fi

checksum_count=$(awk 'NF { count++ } END { print count + 0 }' "$checksums")
if [[ $checksum_count -ne 4 ]]; then
  echo "checksums.txt: expected four archive entries, found $checksum_count" >&2
  exit 1
fi

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  shopt -s nullglob
  archives=("$dist"/devspace_*_"$target".tar.gz)
  shopt -u nullglob
  if [[ ${#archives[@]} -ne 1 ]]; then
    echo "$target: expected one archive, found ${#archives[@]}" >&2
    exit 1
  fi
  archive_name=$(basename "${archives[0]}")
  archive_checksum=$(awk -v name="$archive_name" '$2 == name { count++; checksum = $1 } END { if (count == 1) print checksum }' "$checksums")
  if [[ -z "$archive_checksum" || "$archive_checksum" != "$(sha256 "${archives[0]}")" ]]; then
    echo "$archive_name: missing or invalid checksum" >&2
    exit 1
  fi
  extract="$tmp/$target"
  mkdir -p "$extract"
  tar -xzf "${archives[0]}" -C "$extract"
  for binary in devspace devspace-tui; do
    if [[ ! -f "$extract/$binary" || ! -x "$extract/$binary" ]]; then
      echo "$archive_name: missing executable $binary" >&2
      exit 1
    fi
  done
  echo "$archive_name: devspace devspace-tui"
done
