# 01-spec-cicd-goreleaser.md

## Introduction/Overview

DevDrop currently has no continuous integration and a fully manual release process: a maintainer runs `make verify`, then `make release` and `make checksums`, and hand-uploads archives to GitHub following `docs/release.md`. This feature adds an automated CI/CD pipeline on GitHub Actions: a CI workflow that runs tests, vet, and a build check on every pull request and push to `main`, and a release workflow that runs GoReleaser on version-tag pushes to build multi-platform archives and publish them to GitHub Releases with checksums, a generated changelog, and GitHub artifact attestations.

## Goals

- Every pull request and push to `main` is automatically gated by `go test`, `go vet`, and a successful build (the same checks as `make verify`).
- Pull requests that modify release configuration (`.goreleaser.yaml` or the release workflow) additionally run a GoReleaser snapshot dry-run, so broken release config is caught before a tag is ever pushed.
- Pushing a semver tag (`v*`) automatically produces a GitHub Release containing tar.gz archives for Linux and macOS (amd64 and arm64), a SHA256 checksums file, and a commit-derived changelog — no manual steps.
- Release artifacts carry verifiable build provenance via GitHub artifact attestations (`gh attestation verify` succeeds against the checksums file).
- The GoReleaser configuration passes `goreleaser check` with zero deprecation warnings (config schema `version: 2`).
- The automated release output supersedes the manual `make release` flow, and `docs/release.md` is updated to describe the new tag-driven process.

## User Stories

- **As a maintainer**, I want tests and vet to run automatically on every PR so that regressions are caught before merge instead of relying on contributors remembering to run `make verify`.
- **As a maintainer**, I want to cut a release by pushing a single git tag so that I no longer hand-build archives, compute checksums, and upload files per `docs/release.md`.
- **As a user of DevDrop**, I want to download a prebuilt `devspace` binary for my OS/architecture from GitHub Releases so that I don't need a Go toolchain to install it.
- **As a security-conscious user**, I want to verify that a downloaded archive was genuinely built by this repository's release workflow so that I can trust the binary provenance.

## Demoable Units of Work

### Unit 1: CI Workflow (test, vet, build)

**Purpose:** Gate all pull requests and pushes to `main` with the project's existing quality checks, giving maintainers automatic regression protection.

**Functional Requirements:**

- The system shall provide a GitHub Actions workflow at `.github/workflows/ci.yml` that triggers on pull requests targeting `main` and on pushes to `main`.
- The system shall run `go test ./...`, `go vet ./...`, and a build of `./cmd/devdrop` using the Go version from `go.mod` (via `actions/setup-go` with `go-version-file`).
- The system shall fail the workflow (and thus the PR check) if any test, vet finding, or build error occurs.
- The system shall request only read permissions (`contents: read`) for the CI workflow.

**Proof Artifacts:**

- CI run URL: a green workflow run on a push to `main` (or PR) demonstrates the CI gate executes and passes on the current codebase.
- CLI: `gh run view <run-id>` output showing test/vet/build steps succeeded demonstrates all three checks run in CI.

### Unit 2: GoReleaser Configuration

**Purpose:** Define a reproducible, multi-platform release build that replaces the manual `make release`/`make checksums` targets.

**Functional Requirements:**

- The system shall provide a `.goreleaser.yaml` using config schema `version: 2` that builds the `./cmd/devdrop` package into a binary named `devspace` with `-trimpath` and the existing `-X main.version=...` ldflag wiring (matching `cmd/devdrop/main.go`'s version variable).
- The system shall build exactly four targets: `linux_amd64`, `linux_arm64`, `darwin_amd64`, `darwin_arm64` (Windows is excluded because the `go-fuse` dependency does not compile on Windows).
- The system shall package each target as a tar.gz archive named `devspace_<version>_<os>_<arch>` including `README.md` and `LICENSE` (if present), consistent with the current `make release` layout.
- The system shall generate a SHA256 checksums file and a changelog grouped by conventional-commit prefixes (`feat`, `fix`, others), excluding docs/chore noise.
- The system shall pass `goreleaser check` with no errors and no deprecated fields (e.g., use `archives.formats`, not the removed `archives.format`; no `brews` section).
- The system shall provide a snapshot dry-run workflow at `.github/workflows/release-check.yml` that triggers on pull requests touching `.goreleaser.yaml` or `.github/workflows/release*.yml` and runs `goreleaser release --snapshot --clean`, failing the PR check if the release build would break.

**Proof Artifacts:**

- CLI: `goreleaser check` output reporting a valid config demonstrates schema correctness.
- CLI: `goreleaser release --snapshot --clean` output plus a `dist/` listing showing four `devspace_*` tar.gz archives and a checksums file demonstrates the multi-platform build works end to end locally.
- CI run URL: a green `release-check` run on a PR that touches `.goreleaser.yaml` demonstrates the snapshot dry-run gate fires on release-config changes.

### Unit 3: Tag-Triggered Release Workflow with Attestation

**Purpose:** Turn a pushed version tag into a published GitHub Release with verifiable provenance, completing the automated distribution path.

**Functional Requirements:**

- The system shall provide a GitHub Actions workflow at `.github/workflows/release.yml` that triggers on pushed tags matching `v*`.
- The system shall check out the repository with full history (`fetch-depth: 0`) so GoReleaser can generate the changelog from git history.
- The system shall run GoReleaser via `goreleaser/goreleaser-action` pinned to the current major version, with the GoReleaser version constrained to `~> v2`.
- The system shall grant the workflow `contents: write` (release creation), `id-token: write`, and `attestations: write` (provenance) permissions, and nothing broader.
- The system shall attest the release artifacts by running `actions/attest-build-provenance` against the GoReleaser checksums file after the release publishes.
- The user shall be able to verify a downloaded artifact with `gh attestation verify <file> --repo HexSleeves/devdrop`.
- The system shall mark releases from prerelease-style tags (e.g., `v0.1.0-rc.1`) as GitHub prereleases automatically.

**Proof Artifacts:**

- Release URL: a GitHub Release created automatically from pushing a validation tag (e.g., `v0.1.0-rc.1`, marked prerelease) with four archives + checksums attached demonstrates the tag-to-release path works with no manual steps.
- CLI: `gh attestation verify checksums.txt --repo HexSleeves/devdrop` (or against a downloaded archive) succeeding demonstrates provenance attestation works.
- Workflow run URL: the release workflow run log demonstrates GoReleaser and the attestation step both succeeded under the declared minimal permissions.

## Non-Goals (Out of Scope)

1. **Homebrew tap distribution**: no `homebrew_casks` config, tap repository, or cross-repo PAT in this feature; it can be a follow-up spec.
2. **Docker/GHCR images**: no Dockerfile or container publishing; nothing in the repo motivates container distribution today.
3. **Windows builds**: `go-fuse` does not compile on Windows (verified); enabling Windows requires build-tag work in application code, which is a separate feature.
4. **Adopting golangci-lint**: CI enforces the existing `go test` + `go vet` bar only; introducing a new lint standard (and triaging its findings) is deferred.
5. **Cosign/SLSA signing**: GitHub artifact attestations only; additional signing ecosystems are deferred until a consumer requires them.
6. **Automated version bumping or release PRs**: releases remain maintainer-initiated by pushing a tag; no release-please or tag automation.
7. **Removing the Makefile**: `make build`/`make verify` remain for local development; only the manual release documentation flow is superseded.

## Design Considerations

No user interface is involved. The developer-facing "design" surface is naming and layout: workflow files follow standard GitHub Actions conventions (`.github/workflows/ci.yml`, `.github/workflows/release.yml`), and release archives preserve the existing `devspace_<version>_<os>_<arch>.tar.gz` naming so downloads look identical to the previous manual releases.

## Repository Standards

- **Version injection**: `cmd/devdrop/main.go` receives the version via `-X main.version=...`; GoReleaser must use the same mechanism (its default ldflags template covers this pattern — keep `main.version` as the target).
- **Build flags**: the Makefile builds with `-trimpath`; the GoReleaser build must match.
- **Binary name**: `devspace` (per the Makefile), built from `./cmd/devdrop`.
- **Quality gate**: `make verify` = test → vet → build; CI mirrors exactly this set.
- **Commit conventions**: history uses conventional-commit prefixes (`feat:`, `fix:`); the changelog grouping relies on this.
- **Docs**: release process is documented in `docs/release.md`; that document must be updated to describe the automated flow.

## Technical Considerations

- **GoReleaser v2 schema**: config must declare `version: 2`; use `archives.formats` (plural — `format` was removed in v2.6+) and avoid the removed `brews` key entirely. Validate with `goreleaser check` (current guidance from goreleaser.com deprecations page, mid-2026).
- **Action versions**: `goreleaser/goreleaser-action@v7` with `version: "~> v2"`, `actions/setup-go@v6` with `go-version-file: go.mod` (built-in module caching), `actions/checkout` with `fetch-depth: 0` on the release workflow (required for changelog generation).
- **Attestation**: `actions/attest-build-provenance@v3` pointed at `dist/checksums.txt` artifacts, per GoReleaser's official attestations guidance; requires `id-token: write` + `attestations: write`.
- **CGO**: builds must set `CGO_ENABLED=0` for clean cross-compilation (verified: darwin targets cross-compile from the current codebase with the default toolchain).
- **Existing tags**: the repo has no git tags yet; `git describe` falls back to commit hash. The first automated release establishes the `v*` tag convention. A prerelease tag (`v0.1.0-rc.1`) is the recommended first validation tag so the pipeline can be proven without publishing a stable-looking release.
- **Deviation note**: repository Makefile targets `release`/`checksums` remain but are superseded; keeping them is intentional for offline/manual fallback, with `docs/release.md` pointing to the automated flow as primary.

## Security Considerations

- **No new secrets**: the pipeline uses only the ephemeral `GITHUB_TOKEN`; no PATs, signing keys, or third-party credentials are introduced.
- **Least-privilege permissions**: CI workflow runs with `contents: read`; the release workflow declares only `contents: write`, `id-token: write`, `attestations: write` at the job/workflow level.
- **Provenance**: artifact attestations bind released archives to the exact workflow, commit, and repository that built them, protecting downstream users from tampered binaries.
- **Supply-chain hygiene**: all third-party actions are pinned at least to major versions; no `pull_request_target` triggers; the release workflow runs only on tags pushed by users with write access.

## Success Metrics

1. **CI coverage**: 100% of PRs to `main` and pushes to `main` trigger the CI workflow, and the current codebase passes it green on first run.
2. **Release automation**: one `git push origin <tag>` produces a complete GitHub Release (4 archives + checksums + changelog) with zero manual upload steps, in under ~10 minutes.
3. **Provenance verification**: `gh attestation verify` succeeds against released artifacts for the validation release.
4. **Config hygiene**: `goreleaser check` reports zero errors/deprecations at the merged commit.

## Open Questions

1. The repo has no `LICENSE` file at the root (only README/docs were confirmed); if absent, archives will include only `README.md`. Non-blocking — the archive file list adjusts to what exists.
2. The first validation tag is assumed to be `v0.1.0-rc.1` (prerelease). The maintainer may instead choose to go straight to `v0.1.0`; either works with the same pipeline.
3. `docs/release-readiness.md` is assumed to stay as-is (it documents manual test scenarios, not the release mechanics); only `docs/release.md` is updated.
