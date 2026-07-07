# Plan 019: `devspace tui install` — download the matching devspace-tui release asset into $DEVSPACE_HOME/bin

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 2ff060e..HEAD -- internal/devspace/ui.go internal/devspace/commands.go .goreleaser.yaml internal/devspace/tui_install.go internal/devspace/tui_install_test.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: MED (network code, but read-only against GitHub and atomic on disk)
- **Depends on**: none (pairs well with plan 017's version handshake, but does
  not require it)
- **Category**: dx / direction
- **Planned at**: commit `2ff060e`, 2026-07-07

## Why this matters

`devspace ui` prefers the `devspace-tui` companion (OpenTUI dashboard) and
falls back to the built-in one, printing: "install devspace-tui next to
devspace or in $DEVSPACE_HOME/bin for the full experience". But there is no
install path besides manually finding the right release asset. Releases
already ship per-platform companion binaries (`devspace-tui_<os>_<arch>`,
uploaded by GoReleaser as `extra_files`). One command that downloads the
asset **matching the running devspace version** into `$DEVSPACE_HOME/bin`
makes the good TUI the default experience and eliminates most version-skew
installs. This plan also fixes a release gap it depends on: the tui binaries
are currently **not covered by `checksums.txt`**, so the installer verifies
checksums when available and says so when not.

## Current state

Relevant files and facts:

- `internal/devspace/ui.go` — `findTUIBinary` (lines 58-73) resolves the
  companion: adjacent to the devspace executable → `$DEVSPACE_HOME/bin` →
  PATH. `tuiBinaryName = "devspace-tui"` (line 15). The fallback hint is
  printed at lines 38-39. The install destination this plan targets is the
  second location: `filepath.Join(home, "bin", tuiBinaryName)` where
  `home, _ = appHome()`.
- `internal/devspace/commands.go` — command wiring. `NewRootCommand(version)`
  adds subcommands at lines 36-51 (e.g. `cmd.AddCommand(newUICommand())`,
  line 40). The version string is threaded the same way as
  `newUIServerCommand(version)` (line 41) — mirror that for the new command.
- `cmd/devspace/main.go` — `var version = "dev"` injected by GoReleaser
  ldflags; release versions look like `0.2.0`
  (`.release-please-manifest.json` → `{".": "0.2.0"}`), and release tags are
  `v0.2.0` (release-please default).
- Release assets: `.goreleaser.yaml` `release.extra_files` uploads
  `tui/dist/devspace-tui_*`; names come from `tui/build-all.sh`:
  `devspace-tui_linux_amd64`, `devspace-tui_linux_arm64`,
  `devspace-tui_darwin_amd64`, `devspace-tui_darwin_arm64` (no Windows — the
  companion doesn't build for it).
- `.goreleaser.yaml` checksum block is currently:

```yaml
checksum:
  name_template: checksums.txt
```

  GoReleaser only includes `extra_files` in the checksum file when
  `checksum.extra_files` is configured — so today's `checksums.txt` does NOT
  cover the tui binaries.
- GitHub repo for releases: the Go module is
  `github.com/liatrio-forge/devdrop-capstone` (see `go.mod`) — that is the
  canonical repo the release workflow publishes to. **The repo is private**
  (a recorded project decision), so unauthenticated downloads fail; the
  installer must send a token when one is available.
- HTTP conventions in this repo: `internal/devspace/hosted_sync.go:266` uses
  `&http.Client{Timeout: 30 * time.Second}`. Match that (use a longer timeout
  for the binary download itself — it's ~50-90MB; 5 minutes).
- Atomic file writes: reuse `atomicWriteFile(path, data, perm, backup)` in
  `internal/devspace/jsonio.go:44`. If the binary is too large to buffer
  comfortably, stream to a temp file in the same directory + `os.Rename` —
  follow `atomicWriteFile`'s pattern (same-dir temp, rename).
- Test isolation convention: `t.Setenv("DEVSPACE_HOME", t.TempDir())`.
  `httptest.NewServer` is already used in this package (see
  `hosted_sync`/`hardening` tests) — follow that pattern for stubbing the
  GitHub API.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Tests | `go test ./internal/devspace -run TestTUIInstall -v` | all pass |
| Full gate | `make verify` | exit 0 |
| GoReleaser config check | `go run github.com/goreleaser/goreleaser/v2@latest check` (only if available offline; otherwise skip — CI's release-check workflow validates it) | config valid |
| Manual smoke (optional, needs auth + a real release) | `bin/devspace tui install --version v0.2.0` | binary at $DEVSPACE_HOME/bin/devspace-tui |

## Scope

**In scope** (the only files you should modify/create):
- `internal/devspace/tui_install.go` (create — command + install logic)
- `internal/devspace/tui_install_test.go` (create)
- `internal/devspace/commands.go` (one AddCommand line)
- `internal/devspace/ui.go` (update the fallback hint to mention `devspace tui install`)
- `.goreleaser.yaml` (checksum extra_files)

**Out of scope** (do NOT touch):
- `tui/` — no client change.
- Auto-update / background checks — install is explicit and user-invoked only.
- Windows support — the companion has no Windows build; the command must
  error clearly on `runtime.GOOS == "windows"`.
- Any change to `findTUIBinary` resolution order.

## Git workflow

- Branch: `advisor/019-tui-install`
- Conventional commit, e.g. `feat(ui): devspace tui install downloads the matching companion binary`.
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Cover tui binaries in release checksums

In `.goreleaser.yaml`, extend the checksum block:

```yaml
checksum:
  name_template: checksums.txt
  extra_files:
    - glob: tui/dist/devspace-tui_*
```

**Verify**: `grep -A3 '^checksum:' .goreleaser.yaml` → shows the extra_files glob. (Full validation happens in the release-check CI workflow; do not run a release locally.)

### Step 2: Implement the installer in `internal/devspace/tui_install.go`

Structure (all unexported except the command constructor):

```go
func newTUICommand(version string) *cobra.Command      // parent: Use: "tui"
func newTUIInstallCommand(version string) *cobra.Command
```

`devspace tui install` flags:
- `--version` (string): release tag to install, e.g. `v0.2.0`. Default: `"v" + version`
  of the running binary. If the running version is `dev` and the flag is
  unset, error: `running a dev build; pass --version vX.Y.Z`.
- `--repo` (string): `owner/name`, default `liatrio-forge/devdrop-capstone`
  (define as a const `tuiReleaseRepo`).

Install logic (`tuiInstall(out io.Writer, repo, tag string) error`), decided —
implement exactly this:

1. **Platform**: asset name is
   `fmt.Sprintf("devspace-tui_%s_%s", runtime.GOOS, runtime.GOARCH)`. Allow
   only the four shipped combos (linux/darwin × amd64/arm64); otherwise error
   `no devspace-tui build for <os>/<arch>`.
2. **Token**: first non-empty of `os.Getenv("GITHUB_TOKEN")`,
   `os.Getenv("GH_TOKEN")`, else — if `gh` is on PATH — the trimmed output of
   `gh auth token` (via `exec.CommandContext`, 10s timeout, ignore failure).
   No token is allowed (works if the repo ever goes public) but note it in
   the error path: on HTTP 404, hint
   `release or asset not found (private repo requires GITHUB_TOKEN or gh auth login)`.
3. **Release lookup**: GET
   `https://api.github.com/repos/<repo>/releases/tags/<tag>` with headers
   `Accept: application/vnd.github+json` and, when a token exists,
   `Authorization: Bearer <token>`. Client timeout 30s. Decode only what's
   needed: `{ assets: [ { name, id, url? browser_download_url } ] }` — use the
   asset's **API url** (`url` field) for download, since
   `browser_download_url` does not accept token auth on private repos.
4. **Download**: GET the asset `url` with `Accept: application/octet-stream`
   (plus the token header), client timeout 5 minutes, follow redirects
   (default client behavior; the redirect to storage strips auth — Go's
   http.Client forwards Authorization only to the same host, which is the
   correct behavior; do not override `CheckRedirect`). Stream the body to a
   temp file `devspace-tui.tmp*` created with `os.CreateTemp(destDir, ...)`,
   mode set to `0o755` via `f.Chmod` before rename.
5. **Checksum**: also download `checksums.txt` from the same release (same
   asset-lookup mechanism). If found AND it contains a line for the asset
   name, compute sha256 of the downloaded temp file and compare; mismatch →
   delete temp, error `checksum mismatch for <asset>`. If checksums.txt is
   missing or lacks the asset line, print
   `note: release does not publish a checksum for <asset>; skipping verification`
   and continue (pre-Step-1 releases won't have it).
6. **Destination**: `home, err := appHome()`;
   `dest := filepath.Join(home, "bin", tuiBinaryName)`;
   `os.MkdirAll(filepath.Dir(dest), 0o700)`; `os.Rename(tmp, dest)`.
7. **Report**: print `installed devspace-tui <tag> to <dest>` and, if
   `findTUIBinary()` resolves to a *different* path (e.g. an adjacent binary
   shadows it), warn `note: devspace ui will prefer <other-path>`.

Testability: accept the API base URL as a parameter
(`tuiInstallFrom(out, apiBase, repo, tag string)`) with the production
command passing `https://api.github.com`; tests pass an `httptest.Server`
URL. Keep token lookup in a small `func githubToken() string` you can bypass
in tests by setting `GITHUB_TOKEN` via `t.Setenv`.

**Verify**: `go build ./...` → exit 0.

### Step 3: Wire the command and update the hint

- `commands.go`: add `cmd.AddCommand(newTUICommand(version))` next to the
  other UI wiring (after line 41's `newUIServerCommand`).
- `ui.go` lines 38-39: change the fallback hint to
  `"devspace-tui not found; using the built-in dashboard (run 'devspace tui install' to get the full experience)"`.

**Verify**: `go run ./cmd/devspace tui install --help` → shows the flags; `go run ./cmd/devspace ui --help` still works.

### Step 4: Tests in `tui_install_test.go`

Use `httptest.NewServer` serving a fake GitHub API (pattern: the hosted-sync
tests in this package). Cases:

1. **Happy path with checksum**: fake `/repos/o/r/releases/tags/v1.2.3`
   returning two assets (`devspace-tui_<goos>_<goarch>` and `checksums.txt`)
   with `url` pointing back at the test server; asset endpoint returns bytes
   `fake-binary` when `Accept: application/octet-stream`; checksums.txt
   contains the correct sha256 line. Assert: file exists at
   `$DEVSPACE_HOME/bin/devspace-tui`, mode has `0o111`, content matches.
2. **Checksum mismatch**: wrong hash in checksums.txt → error contains
   `checksum mismatch`, destination file NOT created, no temp file left in
   the dest dir (`os.ReadDir` shows only expected entries).
3. **No checksum coverage**: checksums.txt asset absent → install succeeds,
   output contains `skipping verification`.
4. **Missing asset**: release JSON has no matching asset → error names the
   asset and platform.
5. **Auth header**: `t.Setenv("GITHUB_TOKEN", "test-token")` (fake value) →
   handler asserts `Authorization: Bearer test-token` was received.
6. **Unsupported platform**: call the platform-validation helper directly
   with `windows/amd64` → error. (Factor the asset-name/validation into a
   pure func so this doesn't need build tags.)
7. **Dev version guard**: command with version `dev` and no `--version` →
   error mentions `--version`.

All tests set `t.Setenv("DEVSPACE_HOME", t.TempDir())`.

**Verify**: `go test ./internal/devspace -run TestTUIInstall -v` → all 7 pass.

### Step 5: Full gate

**Verify**: `make verify` → exit 0 (lint will check the new file; match repo comment style — terse, why-focused).

## Test plan

Covered in Step 4. No network access in tests — everything through
`httptest`. The one thing tests cannot cover is real GitHub auth/redirect
behavior; that's the optional manual smoke in "Commands you will need" and
should be run once by a human with repo access before the next release notes
mention the command.

## Done criteria

- [ ] `make verify` exits 0
- [ ] `go test ./internal/devspace -run TestTUIInstall -v` → 7 tests pass
- [ ] `.goreleaser.yaml` checksum block includes the tui glob
- [ ] `go run ./cmd/devspace tui install --help` documents `--version` and `--repo`
- [ ] The `devspace ui` fallback hint mentions `devspace tui install`
- [ ] No secret values in any file (token comes from env/gh at runtime only)
- [ ] No files outside the in-scope list modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- `appHome()` or `atomicWriteFile` signatures differ from "Current state"
  (drift in paths.go/jsonio.go).
- You find an existing download/HTTP helper in the package that overlaps —
  reuse it instead, and note the deviation; if reuse would change its
  behavior for other callers, STOP.
- The GitHub API contract described in Step 2 doesn't hold in the manual
  smoke (e.g. asset `url` + octet-stream doesn't redirect as described) —
  report; do not iterate on live network behavior beyond one retry.
- You are tempted to add auto-update, version pinning files, or signature
  verification — explicitly deferred (see Maintenance notes).

## Maintenance notes

- Deferred on purpose: auto-update checks, artifact attestation verification
  (`gh attestation verify` — the repo already produces attestations; a
  follow-up could verify them when `gh` is present), and Windows support
  (needs a tui build first).
- If the repo goes public, nothing changes — the token becomes optional in
  practice, and the 404 hint stops being the common failure.
- When plan 017's handshake lands, a protocol-mismatch error in the TUI plus
  this command is the complete skew story: the error message should
  eventually suggest `devspace tui install` (one-line follow-up, not done
  here to keep plans independent).
- Reviewer scrutiny: temp-file cleanup on every error path (use
  `defer os.Remove(tmpName)` and let the successful rename make it a no-op),
  and that no token value can ever end up in an error message or log line.
