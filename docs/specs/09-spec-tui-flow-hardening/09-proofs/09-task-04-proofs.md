# Task 04 Proofs - `devspace tui install` downloads the version-matched companion with checksum verification

## Task Summary

This task shipped the missing install path for the `devspace-tui` companion,
per `plans/019-devspace-tui-install-command.md` (executed by Codex under
orchestrator review; lint findings fixed and gates run by the orchestrator).
`devspace tui install` resolves the release tag from the running binary's
version (`--version` overrides; required on `dev` builds), downloads the
`devspace-tui_<os>_<arch>` asset via the GitHub releases API (Bearer token
from `GITHUB_TOKEN`/`GH_TOKEN`/`gh auth token` when available — the canonical
repo is private), verifies its sha256 against the release's `checksums.txt`
when covered (and says so when not), and installs it atomically (same-dir
temp + chmod 0755 + rename) to `$DEVSPACE_HOME/bin/devspace-tui`. The
GoReleaser checksum config now includes the tui binaries so future releases
are verifiable, and the `devspace ui` fallback hint points at the new command.

## What This Task Proves

- The full install flow works against a faked GitHub API without network
  access: release lookup, asset selection, authenticated download, checksum
  verification, atomic install.
- Failure modes behave as specified: checksum mismatch leaves no destination
  file and no temp litter; a missing asset names the asset and platform;
  unsupported platforms error clearly; `dev` builds require `--version`.
- No token value can leak: the happy-path test asserts output and errors never
  contain the placeholder token; the auth test asserts the header arrives as
  `Bearer test-token` (placeholder value only, per repo security guidance).
- Future releases publish tui checksums (`.goreleaser.yaml` `checksum.extra_files`).

## Evidence Summary

- All 7 httptest-backed install tests pass.
- `go run ./cmd/devspace tui install --help` documents `--version` and
  `--repo` with correct defaults.
- `checksum:` block in `.goreleaser.yaml` now globs `tui/dist/devspace-tui_*`.
- `make verify` exit 0 after fixing 4 lint findings (3 errcheck, 1 gosec) in
  the new files.

## Artifact: Seven httptest-backed install tests

**What it proves:** Every Unit 4 functional requirement, without network.

**Command:**

~~~bash
go test ./internal/devspace -run TestTUIInstall -v
~~~

**Result summary:** 7/7 pass.

~~~text
--- PASS: TestTUIInstallHappyPathWithChecksum (0.02s)
--- PASS: TestTUIInstallChecksumMismatchCleansUp (0.01s)
--- PASS: TestTUIInstallNoChecksumCoveragePrintsSkippingVerification (0.01s)
--- PASS: TestTUIInstallMissingAssetNamesAssetAndPlatform (0.00s)
--- PASS: TestTUIInstallSendsAuthHeader (0.01s)
--- PASS: TestTUIInstallUnsupportedPlatform (0.00s)
--- PASS: TestTUIInstallDevVersionRequiresVersionFlag (0.00s)
ok  	github.com/liatrio-forge/devdrop-capstone/internal/devspace	0.047s
~~~

## Artifact: Command help

**What it proves:** The command, flags, and defaults exist and are documented.

**Command:**

~~~bash
go run ./cmd/devspace tui install --help
~~~

**Result summary:** Shows `--version` (release tag, defaulting from the
running binary's version) and `--repo` (default `liatrio-forge/devdrop-capstone`).

~~~text
  USAGE
    devspace tui install [--flags]

  FLAGS
    -h --help   Help for install
    --no-color  Disable styled output regardless of terminal capability
    --repo      Github repository owner/name (liatrio-forge/devdrop-capstone)
    --version   Release tag to install, e.g. v0.2.0 (vdev)
~~~

(Default shows `vdev` because this help run used a source build; the RunE
guard rejects `dev` builds without an explicit `--version`, covered by
`TestTUIInstallDevVersionRequiresVersionFlag`.)

## Artifact: Release checksum coverage

**What it proves:** Future releases include tui binaries in `checksums.txt`,
so the installer's verification path is the norm going forward.

**Command:**

~~~bash
grep -B1 -A3 '^checksum:' .goreleaser.yaml
~~~

~~~yaml
checksum:
  name_template: checksums.txt
  extra_files:
    - glob: tui/dist/devspace-tui_*
~~~

## Artifact: Discovery hint

**What it proves:** Users without the companion learn about the install path.

**Result summary:** The `devspace ui` fallback line now reads
"devspace-tui not found; using the built-in dashboard (run 'devspace tui
install' to get the full experience)" (`internal/devspace/ui.go`).

## Artifact: Full gate

**Command:**

~~~bash
make verify
~~~

**Result summary:** Exit 0 (test + vet + lint + vulncheck + build). Four lint
findings in the new files (unchecked `os.Remove`/`f.Close`/`w.Write`, gosec
G204 on the fixed-args `gh auth token` invocation) were fixed/annotated by
the orchestrator before commit.

## Reviewer Conclusion

Unit 4's functional requirements are implemented and proven without network
access. The remaining item, by design, is the one-time manual smoke against a
real release by a human with repo access (`devspace tui install --version
v0.2.0`) before the command is announced in release notes — note that
releases cut before this change do not include tui checksums, so the
"skipping verification" path is expected for v0.2.0.
