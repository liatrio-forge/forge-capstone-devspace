#!/usr/bin/env bash
# Cross-compile devspace-tui for every release platform via Bun's
# single-file executable targets. Output names match GoReleaser's
# <os>_<arch> convention so release assets line up with the Go binaries.
set -euo pipefail
cd "$(dirname "$0")"

# @opentui/core ships its native module as os/cpu-gated optional deps, so a
# plain `bun install` only materializes the host's package. Force every
# target platform's native package into node_modules (frozen, no lockfile
# drift) so cross-compiling below can resolve them all.
bun install --frozen-lockfile --os '*' --cpu '*'

declare -A targets=(
  [linux_amd64]=bun-linux-x64
  [linux_arm64]=bun-linux-arm64
  [darwin_amd64]=bun-darwin-x64
  [darwin_arm64]=bun-darwin-arm64
)

mkdir -p dist
for name in "${!targets[@]}"; do
  echo "building dist/devspace-tui_${name}"
  bun build --compile --target="${targets[$name]}" --outfile "dist/devspace-tui_${name}" src/main.tsx
done
