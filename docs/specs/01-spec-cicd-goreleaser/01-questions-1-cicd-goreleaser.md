# 01 Questions Round 1 - CI/CD Pipeline with GoReleaser

Please answer each question below (select one or more options, or add your own notes). Feel free to add additional context under any question.

> **Answer provenance:** The user directed the workflow to continue autonomously through all SDD phases (`/loop continue through the whole SDD process`), so the recommended answer for each question below was auto-selected on 2026-07-01. Each selection follows the written recommendation rationale.

## 1. Pipeline Scope

Should this spec cover only the release pipeline, or also continuous integration on pull requests and pushes?

- [ ] (A) Release pipeline only: a workflow that runs GoReleaser when a version tag is pushed
- [ ] (B) Full CI + release: a CI workflow (test, vet/lint, build) on PRs and pushes to `main`, plus the tag-triggered release workflow
- [x] (C) Full CI + release, and the release workflow also runs a GoReleaser snapshot dry-run on PRs that touch release config
- [ ] (E) Other (describe)

**Current best-practice context:** GoReleaser's official guidance is a tag-push-triggered release workflow; most projects pair it with a separate CI workflow, and many add a `goreleaser build --snapshot` check on PRs so release config breakage is caught before tagging.

**Recommended answer(s):** [(B)]

**Why these are recommended:**

- The repo currently has no CI at all, so "CI/CD pipeline" most plausibly means both halves; shipping release automation with no PR-time test gate would leave `make verify` as a purely manual step.
- `(B)` keeps the spec bounded: two workflow files plus one GoReleaser config. `(C)` adds the snapshot dry-run, which is a nice safety net but can be added later as a one-line job; choose it if you want maximum protection against broken releases from day one.
- `(A)` is the smallest slice but leaves tests unenforced on PRs, which undercuts the value of a "CI/CD pipeline".

## 2. Distribution Channels

Where should released artifacts be published? GitHub Releases (tar.gz archives + checksums) is the baseline that matches your current `make release` output.

- [x] (A) GitHub Releases only: multi-platform archives, checksums, and generated changelog
- [ ] (B) GitHub Releases + Homebrew tap (requires creating a `HexSleeves/homebrew-tap` repo and a PAT secret, since the default `GITHUB_TOKEN` cannot push to another repo)
- [ ] (C) GitHub Releases + Docker image on GHCR (requires adding a Dockerfile and `packages: write` permission)
- [ ] (D) All of the above
- [ ] (E) Other (describe)

**Current best-practice context:** GoReleaser removed the old `brews` config in v2.16 (May 2026); Homebrew distribution now uses `homebrew_casks` and still requires a cross-repo PAT for a separate tap repo. Docker/GHCR is fully supported OSS but needs a Dockerfile, which the repo does not have.

**Recommended answer(s):** [(A)]

**Why these are recommended:**

- `(A)` is a complete, demoable release pipeline with zero new credentials or repos — it exactly automates what `make release` + `docs/release.md` do by hand today.
- `(B)` is the natural next increment for a developer CLI, but it introduces a PAT secret and a second repository; better as a follow-up spec unless you already want Homebrew installs now.
- `(C)`/`(D)` add container distribution that nothing in the repo currently motivates (DevDrop is a local-first CLI that mounts FUSE filesystems — awkward in a container).

## 3. Artifact and Project Naming

The Go module is `github.com/HexSleeves/devdrop`, but the Makefile builds the binary as `devspace`, and there is a rename engineering review in the repo. What name should release artifacts use?

- [x] (A) `devspace` everywhere: binary name and archive names (`devspace_v0.1.0_darwin_arm64.tar.gz`), matching the current Makefile
- [ ] (B) `devdrop` everywhere: binary and archives match the repo/module name
- [ ] (C) Binary `devspace`, archives named after the repo `devdrop`
- [ ] (E) Other (describe)

**Recommended answer(s):** [(A)]

**Why these are recommended:**

- `(A)` matches the existing Makefile output (`bin/devspace`, `dist/devspace_<VERSION>_...`) and `docs/release.md`, so the automated pipeline produces artifacts users already recognize.
- `(B)` would be right only if the rename direction is actually *toward* `devdrop`; the Makefile suggests the opposite. `(C)` creates a confusing mismatch between what users download and what they run.
- If the rename decision is still genuinely open, answer `(E)` with the intended final name so the pipeline doesn't have to be renamed immediately after shipping.

## 4. Target Platforms

Which OS/architecture combinations should releases build for? Note: I verified that `GOOS=windows` currently fails to compile because the `go-fuse` dependency is Unix-only, so Windows support would require code changes (build tags) that are out of scope for a CI/CD spec.

- [x] (A) Linux and macOS, both amd64 and arm64 (4 artifacts)
- [ ] (B) Linux and macOS amd64/arm64, plus FreeBSD
- [ ] (C) Linux only (amd64 + arm64)
- [ ] (E) Other (describe)

**Recommended answer(s):** [(A)]

**Why these are recommended:**

- `(A)` covers the realistic user base for a developer workstation CLI and is exactly what cross-compiles cleanly today (verified: darwin/arm64 builds; windows does not).
- `(C)` is too narrow given macOS is a primary developer platform; `(B)` adds a platform with no stated demand and no test coverage.
- Windows can become its own future spec (guard the FUSE mount behind build tags first).

## 5. Artifact Signing and Provenance

Should release artifacts be signed or attested for supply-chain integrity?

- [ ] (A) None for now: checksums file only (matches current manual process)
- [x] (B) GitHub artifact attestations: `actions/attest-build-provenance` over the checksums file — zero key management, verifiable with `gh attestation verify`
- [ ] (C) GitHub attestations + cosign keyless signing for broader ecosystem verification
- [ ] (E) Other (describe)

**Current best-practice context:** The pragmatic 2026 default for small OSS CLIs is GitHub native artifact attestations (keyless, built into Actions, needs only `id-token: write` + `attestations: write` permissions). Cosign adds compatibility with non-GitHub verifiers; full SLSA generators are considered overkill at this scale.

**Recommended answer(s):** [(B)]

**Why these are recommended:**

- `(B)` is nearly free to add (a few lines, no secrets, no key rotation) and upgrades your existing SHA256SUMS practice into verifiable provenance.
- `(A)` is acceptable for a prototype, but since you already generate checksums manually, attestation is the modern equivalent of that same instinct.
- `(C)` adds a second tool and verification story; worth it only if you expect users to verify with cosign specifically.

## 6. Linting in CI

The repo currently uses only `go vet` (via `make vet`). What should the CI quality gate run?

- [x] (A) Keep it as-is: `go test` + `go vet`, mirroring `make verify`
- [ ] (B) Add golangci-lint: introduce a `.golangci.yml` and run `golangci-lint-action` in CI alongside tests
- [ ] (E) Other (describe)

**Current best-practice context:** golangci-lint (action v9) is the de facto standard Go CI linter, but introducing it to an existing codebase usually surfaces a batch of findings that need triage or config tuning — a real scope expansion.

**Recommended answer(s):** [(A)]

**Why these are recommended:**

- `(A)` keeps the spec focused on pipeline automation: CI enforces exactly what `make verify` already promises, so the pipeline lands green on day one.
- `(B)` is worthwhile but mixes "set up CI/CD" with "adopt a new lint standard and fix its findings" — cleaner as its own follow-up spec. Choose it if you want the lint bar raised now and accept fixing findings as part of this work.
