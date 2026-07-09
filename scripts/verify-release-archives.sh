#!/usr/bin/env bash
set -euo pipefail

dist=${1:-dist}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  shopt -s nullglob
  archives=("$dist"/devspace_*_"$target".tar.gz)
  shopt -u nullglob
  if [[ ${#archives[@]} -ne 1 ]]; then
    echo "$target: expected one archive, found ${#archives[@]}" >&2
    exit 1
  fi
  extract="$tmp/$target"
  mkdir -p "$extract"
  tar -xzf "${archives[0]}" -C "$extract"
  for binary in devspace devspace-tui; do
    if [[ ! -f "$extract/$binary" || ! -x "$extract/$binary" ]]; then
      echo "$(basename "${archives[0]}"): missing executable $binary" >&2
      exit 1
    fi
  done
  echo "$(basename "${archives[0]}"): devspace devspace-tui"
done
