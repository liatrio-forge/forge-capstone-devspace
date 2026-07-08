# 12-spec-tui-build-all-cross-compile.md

## Introduction/Overview

`make tui-build-all` cross-compiles the `devspace-tui` companion for all four
release platforms from a single host, but it fails because `@opentui/core`
ships its native library as `os`/`cpu`-gated optional dependencies and
`bun install` only materializes the one matching the build host. Cross-compiling
to any other platform then fails with `Could not resolve
"@opentui/core-<os>-<arch>"`. This spec makes `make tui-build-all` reliably
produce all four platform binaries on any single host (developer macOS **and**
the `ubuntu-latest` release runner) with a minimal, verified change.

## Goals

- Make `make tui-build-all` exit 0 and emit all four
  `tui/dist/devspace-tui_<os>_<arch>` binaries from a single host, regardless of
  which OS/arch that host is.
- Fix the failure symmetrically for CI: the same command must succeed on
  `ubuntu-latest` (where the *darwin* targets would otherwise fail to resolve).
- Keep the change minimal and lockfile-stable — no new runtime dependencies, no
  lockfile drift, no CI runner matrix.
- Ensure the GoReleaser release path (`release.yml`) and the PR dry-run
  (`release-check.yml`, per plan 021) can attach the four TUI assets.

## User Stories

- **As a maintainer cutting a release**, I want `make tui-build-all` to build
  every platform binary on the Linux release runner so that GoReleaser can
  attach and checksum the `devspace-tui_*` assets and the companion actually
  ships.
- **As a developer on macOS**, I want `make tui-build-all` to succeed locally so
  that I can produce and test the release assets without a Linux machine.
- **As a devspace user**, I want `devspace tui install --version vX.Y.Z` to find
  a `devspace-tui_<os>_<arch>` asset on the next release so that the companion
  installs instead of erroring with "release asset ... not found".

## Demoable Units of Work

### Unit 1: All four platform binaries build from one host

**Purpose:** Fix the core cross-compile resolution failure so a single host
produces every release binary.

**Functional Requirements:**
- The system shall install every target platform's `@opentui/core-*` native
  optional dependency into `node_modules` before cross-compiling, on whatever
  host runs the build.
- The system shall continue to install dependencies with a frozen lockfile so no
  lockfile mutation occurs (all eight platform packages are already recorded in
  `tui/bun.lock`).
- `make tui-build-all` shall produce `tui/dist/devspace-tui_linux_amd64`,
  `devspace-tui_linux_arm64`, `devspace-tui_darwin_amd64`, and
  `devspace-tui_darwin_arm64`, each a valid executable for its declared OS/arch.

**Proof Artifacts:**
- CLI: `make tui-build-all` exits 0 demonstrates the build no longer fails on the
  missing-native-package error.
- CLI: `file tui/dist/devspace-tui_*` shows `ELF ... x86-64`, `ELF ... ARM
  aarch64`, `Mach-O ... x86_64`, `Mach-O ... arm64` respectively demonstrates
  each target compiled to the correct architecture.
- CLI: `git diff --exit-code tui/bun.lock` after the build exits 0 demonstrates
  the frozen lockfile did not drift.

### Unit 2: Host-native binary runs (smoke test)

**Purpose:** Confirm a produced binary is not merely emitted but actually starts,
without requiring foreign-architecture runtime infrastructure.

**Functional Requirements:**
- The system shall produce a binary for the build host's own platform that
  starts and responds to `--help`.

**Proof Artifacts:**
- CLI: on a darwin-arm64 host, `tui/dist/devspace-tui_darwin_arm64 --help` prints
  the companion usage/help text demonstrates the host-native artifact is
  runnable, not just linkable.
- CLI: `devspace ui` launches the companion when
  `~/.devspace/bin/devspace-tui` is the host-native build demonstrates the
  end-to-end install path works.

## Non-Goals (Out of Scope)

1. **Windows targets**: GoReleaser ships only linux and darwin archives, so
   `win32` binaries are not built despite OpenTUI publishing native packages for
   them.
2. **CI runner matrix**: Building each platform on its own native runner is
   explicitly rejected; research shows the single-host `--os '*' --cpu '*'`
   install resolves the failure without matrix complexity.
3. **Foreign-architecture runtime validation**: Executing the linux and
   opposite-arch binaries on their real target platforms (which needs cross-arch
   CI/emulation) is deferred; this spec validates *build* correctness plus a
   host-native smoke test only.
4. **Upstream OpenTUI runtime bugs**: Any `bun build --compile` runtime defect in
   OpenTUI itself (e.g. Worker/tree-sitter bundling, `sst/opentui#807`) is not
   fixed here.
5. **Re-releasing v0.2.0 or changing publish/tag/deploy behavior**: The existing
   release, tagging, and Railway deploy flows are untouched.
6. **`devspace tui install` code changes**: The Go install command is already
   correct; it only fails because past releases carried no TUI asset.

## Design Considerations

No specific design requirements identified. This is a build/CI change with no
user-facing UI surface beyond the eventual availability of the companion binary.

## Repository Standards

- Follow the existing `tui/build-all.sh` convention: output names must match
  GoReleaser's `<os>_<arch>` template (`.goreleaser.yaml` globs
  `tui/dist/devspace-tui_*`).
- Preserve the pinned Bun version (`1.3.14`) used in `release.yml`,
  `release-check.yml`, and `oven-sh/setup-bun`.
- Keep `bun install` frozen (`--frozen-lockfile`), matching the existing
  `Makefile` `tui-install` target and CI conventions.
- Conventional-commit style for the change (e.g. `fix(tui): install all platform
  natives before cross-compile`).
- Do not commit generated `tui/dist/` output; it is ignored build product.

## Technical Considerations

- **Root cause**: `@opentui/core@0.4.3` declares eight `@opentui/core-<os>-<arch>`
  native packages as `os`/`cpu`-gated `optionalDependencies`. `bun install`
  materializes only the host-matching package, so `bun build --compile
  --target=<foreign>` cannot resolve the foreign platform's native module.
  `--target` changes only the output runtime/ABI; module resolution is a
  host-side step against whatever is present in `node_modules`.
- **Fix (verified)**: run `bun install --frozen-lockfile --os '*' --cpu '*'`
  before cross-compiling. This flag (Bun ≥ 1.2.23; repo uses 1.3.14) forces all
  platforms' gated optional deps into `node_modules` without lockfile changes.
  Verified locally: after this install, all four targets in `build-all.sh`
  compile to correct-architecture binaries on a darwin-arm64 host.
- **Placement**: the all-platform install belongs with the script that
  cross-compiles. Recommended: make `tui/build-all.sh` self-contained by running
  the all-platform install at its start (the script that needs foreign natives
  owns that requirement), or apply the flag to the `Makefile` install step that
  `tui-build-all` depends on. The narrowest change that makes `make
  tui-build-all` succeed on any host is preferred; exact placement is an
  implementation decision for the task phase.
- **CI parity**: wherever `make tui-build-all` runs (`release.yml`, and
  `release-check.yml` once plan 021 lands), the same all-platform install must
  take effect, or `ubuntu-latest` will hit the mirror-image
  `Could not resolve "@opentui/core-darwin-*"` failure.
- **Coordination with plan 021**: plan 021 adds Bun setup + `make tui-build-all`
  to `release-check.yml`. That dry-run can only pass once this fix makes
  `tui-build-all` work; the two are complementary.

## Security Considerations

- No new dependencies are introduced. The all-platform install pulls only
  packages already pinned in `tui/bun.lock`; `--frozen-lockfile` prevents any
  lockfile drift or unpinned resolution.
- No secrets, tokens, or credentials are involved in the build step.
- Generated binaries under `tui/dist/` must not be committed (already
  `.gitignore`d); only release assets published by GoReleaser are distributed.

## Success Metrics

1. **Build success**: `make tui-build-all` exits 0 on both a macOS host and
   `ubuntu-latest`, producing all four `devspace-tui_*` binaries (target: 4/4).
2. **Architecture correctness**: `file` reports the correct OS/arch for each of
   the four binaries (target: 4/4 correct).
3. **Lockfile stability**: `git diff --exit-code tui/bun.lock` is clean after the
   build (target: no drift).
4. **Release readiness**: a GoReleaser snapshot/release run attaches four
   `devspace-tui_*` assets and includes them in `checksums.txt` (target: 4 assets
   + checksum entries).

## Open Questions

1. **Verification depth (non-blocking)**: This spec validates build correctness
   plus a host-native smoke test. Full runtime validation of the linux and
   opposite-arch binaries on their real targets is deferred as a follow-up; if
   desired, it would require cross-arch CI or emulation and can be added without
   changing this fix. Assumption: build + host-native smoke test is sufficient
   proof for this bug.
2. **Flag placement (non-blocking)**: `build-all.sh` self-contained install vs.
   `Makefile` `tui-install` target — both satisfy the requirement; the task phase
   picks the narrowest one. This does not affect scope or acceptance criteria.
3. **OpenTUI `--compile` runtime risk (non-blocking, upstream)**: `sst/opentui#807`
   reports Worker/tree-sitter bundling issues in relocated compiled binaries.
   This is a pre-existing upstream risk, not introduced by this fix, and is out
   of scope; noted so validation is not surprised if markdown highlighting
   degrades on a relocated binary.
