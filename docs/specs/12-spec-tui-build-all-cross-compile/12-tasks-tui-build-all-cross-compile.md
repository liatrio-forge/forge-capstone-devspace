# 12-tasks-tui-build-all-cross-compile.md

> Task list for spec `12-spec-tui-build-all-cross-compile`.

## Relevant Files

| File | Why It Is Relevant |
| --- | --- |
| `tui/build-all.sh` | The cross-compile script. Gains a `bun install --frozen-lockfile --os '*' --cpu '*'` line before the loop so all target-platform `@opentui/core-*` natives are resolvable. This is the fix. |
| `Makefile` | Defines `tui-build-all: tui-install` (`tui-install` = `bun install --frozen-lockfile`). Reference only — no edit needed; the redundant host-only install before `build-all.sh` is idempotent. |
| `tui/bun.lock` | Must stay unchanged; already records all 8 platform packages, so `--frozen-lockfile` is compatible. Proof surface, not edited. |
| `.goreleaser.yaml` | Globs `tui/dist/devspace-tui_*` into release + checksum extra_files. Reference only — validates the four assets attach once the build succeeds. |
| `.github/workflows/release.yml` | Runs `make tui-build-all` on `ubuntu-latest`; the fix makes that step succeed there too (darwin targets). Reference only — no edit. |
| `.github/workflows/release-check.yml` | PR dry-run. Adding Bun + `make tui-build-all` here is **plan 021's** scope, out of scope for this spec; noted for coordination. |

### Notes

- The change is a shell/Makefile build fix; there is no unit-test harness for
  `build-all.sh`. The runnable check is `make tui-build-all` followed by `file`
  on each output binary (the arch assertion) — these are the proof artifacts, not
  a separate test file.
- Do not commit generated `tui/dist/` or `dist/` output (git-ignored per
  `AGENTS.md`).
- Use Conventional Commit style, e.g. `fix(tui): install all platform natives
  before cross-compile`. In the PR, mention the affected release workflow and
  GoReleaser asset impact (per `AGENTS.md`).

## Tasks

### [ ] 1.0 Install all target-platform native deps before cross-compiling

Make the `tui-build-all` path force every target platform's `@opentui/core-*`
native optional dependency into `node_modules` (via `bun install --frozen-lockfile
--os '*' --cpu '*'`) so a single host can cross-compile all four release binaries.
Covers spec Unit 1 (all FRs) and Unit 2 (host-native smoke test).

#### 1.0 Proof Artifact(s)

- CLI: `make tui-build-all` exits 0 and emits `tui/dist/devspace-tui_linux_amd64`,
  `_linux_arm64`, `_darwin_amd64`, `_darwin_arm64` demonstrates the resolve
  failure is fixed.
- CLI: `file tui/dist/devspace-tui_*` reports `ELF ... x86-64`, `ELF ... ARM
  aarch64`, `Mach-O ... x86_64`, `Mach-O ... arm64` respectively demonstrates each
  target compiled to the correct architecture.
- CLI: `git -C tui diff --exit-code bun.lock` exits 0 demonstrates the frozen
  lockfile did not drift.
- CLI: on a darwin-arm64 host, `tui/dist/devspace-tui_darwin_arm64 --help` prints
  the companion usage text demonstrates the host-native artifact runs.

#### 1.0 Tasks

- [ ] 1.1 In `tui/build-all.sh`, add `bun install --frozen-lockfile --os '*'
  --cpu '*'` immediately after the `cd "$(dirname "$0")"` line and before the
  `mkdir -p dist` / build loop, with a one-line comment explaining that
  `@opentui/core` ships `os`/`cpu`-gated native optional deps that must all be
  present for cross-compile resolution.
- [ ] 1.2 Run `make tui-build-all`; confirm exit 0 and that all four
  `tui/dist/devspace-tui_*` files are produced.
- [ ] 1.3 Run `file tui/dist/devspace-tui_linux_amd64 tui/dist/devspace-tui_linux_arm64
  tui/dist/devspace-tui_darwin_amd64 tui/dist/devspace-tui_darwin_arm64`; confirm
  each reports its correct OS/arch (ELF x86-64, ELF ARM aarch64, Mach-O x86_64,
  Mach-O arm64).
- [ ] 1.4 Run `git -C tui diff --exit-code bun.lock`; confirm exit 0 (no lockfile
  drift). If it drifts, STOP — the `--os '*'` install must not rewrite the lock.
- [ ] 1.5 Smoke-test the host-native binary: run
  `tui/dist/devspace-tui_darwin_arm64 --help` (on a darwin-arm64 host) and confirm
  it prints the companion usage text.
- [ ] 1.6 Run `make tui-verify`; confirm typecheck + tests still pass (the change
  must not regress the host-only build path).

### [ ] 2.0 Verify the GoReleaser release path attaches all four TUI assets

Confirm the fix holds where `make tui-build-all` runs in CI (`release.yml` on
`ubuntu-latest`), so GoReleaser attaches and checksums the four `devspace-tui_*`
assets. Dry-run/snapshot only — no publishing, tagging, or deploy. Covers the
spec goal "Ensure the GoReleaser release path and PR dry-run can attach the four
TUI assets."

#### 2.0 Proof Artifact(s)

- CLI: `goreleaser check` exits 0 (`1 configuration file(s) validated`)
  demonstrates `.goreleaser.yaml` remains valid with the `devspace-tui_*` globs.
- CLI: `goreleaser release --snapshot --clean --skip=publish` produces `dist/`
  with the four `devspace-tui_*` files and a `checksums.txt` listing them; OR a
  documented Docker-unavailable skip (ko image build) plus the green CI
  `release-check` run stands in.
- CLI: `grep -c 'devspace-tui_' dist/checksums.txt` returns `4` demonstrates all
  four companion binaries are checksummed alongside the Go archives.

#### 2.0 Tasks

- [ ] 2.1 Run `goreleaser check`; confirm `1 configuration file(s) validated`.
- [ ] 2.2 If `docker info` succeeds, run `goreleaser release --snapshot --clean
  --skip=publish` and confirm `dist/` contains the four `devspace-tui_*` files and
  `grep -c 'devspace-tui_' dist/checksums.txt` returns `4`. If Docker is
  unavailable, do not install it — record the skip note and rely on CI
  `release-check`.
- [ ] 2.3 Confirm no generated `tui/dist/` or `dist/` output is staged for commit
  (`git status --porcelain` shows none) — build output stays ignored.
- [ ] 2.4 Confirm `release.yml` already invokes `make tui-build-all` so no
  workflow edit is required for this spec; note that adding the same step to
  `release-check.yml` is plan 021's scope, not this task.
