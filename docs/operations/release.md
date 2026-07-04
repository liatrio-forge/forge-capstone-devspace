# DevDrop release packaging

DevDrop ships the local-first CLI as a `devspace` binary built from the existing
Go module. Releases are **automated with GoReleaser**: pushing a version tag
builds multi-platform archives and publishes them to GitHub Releases with
checksums, a generated changelog, and build-provenance attestation. GoReleaser is
the single source of truth for release artifacts (there is no separate manual
release path). This workflow packages the CLI; the hosted server is published
separately as a container image. It does not add a daemon, FUSE behavior, managed
team identity, or dependency install behavior.

## Automated release (primary)

### How it works

- `.github/workflows/ci.yml` runs `go test`, `go vet`, and a build on every PR
  and push to `main`.
- `.github/workflows/release-check.yml` runs a GoReleaser snapshot dry-run on
  PRs that touch `.goreleaser.yaml` or the release workflows.
- `.github/workflows/release-please.yml` maintains an automatic **release PR**
  (next version + `CHANGELOG.md`) from conventional commits; merging it pushes
  the `vX.Y.Z` tag.
- `.github/workflows/release.yml` runs GoReleaser when a `v*` tag is pushed
  (archives, checksums, the ghcr image, attestation), then a **gated
  `deploy-railway` job** waits for manual approval on the `production`
  environment before deploying the image to Railway (stable tags only). That
  job first mirrors the image from `ghcr.io/liatrio-forge/devspace-hosted`
  (org, permanently private — see prerequisites) to
  `ghcr.io/hexsleeves/devspace-hosted` (personal, public) and deploys the
  mirror, since Railway can't pull a private image on our plan.

Each release contains four tar.gz archives (`linux`/`darwin` × `amd64`/`arm64`)
named `devspace_<version>_<os>_<arch>.tar.gz`, plus a `checksums.txt` file
(SHA256). Windows is not built: the `go-fuse` dependency does not compile on
Windows. Tags with a prerelease suffix (for example `v0.1.0-rc.1`) are marked
as GitHub prereleases automatically.

### Cutting a release

Versioning is automatic — you do not tag by hand.

1. Land feature PRs on `main` with conventional-commit titles (`feat:`/`fix:`).
2. `release-please` opens/updates a **release PR** with the next version and
   `CHANGELOG.md`. Review and **merge it** when you want to ship.
3. Merging the release PR pushes the `vX.Y.Z` tag, which runs `release.yml`:
   GoReleaser creates the GitHub Release with the archives, `checksums.txt`, and
   the ghcr image (changelog from `feat:`/`fix:` history).
4. For a **stable** tag, the `deploy-railway` job then pauses at **"Waiting for
   review"** on the `production` environment — approve it in the Actions run to
   deploy the image to Railway. Prerelease (`-rc`) tags skip the deploy.

To publish binaries without a live deploy (e.g. a preview), push a prerelease
tag by hand: `git tag v0.1.0-rc.3 && git push origin v0.1.0-rc.3`.

### Setup prerequisites (one-time)

The automated flow above depends on repo configuration that must exist first,
or release runs will fail:

- **`RELEASE_PLEASE_TOKEN`** repo secret — a fine-grained PAT with **Contents:
  write** + **Pull requests: write**. It must NOT be the default `GITHUB_TOKEN`,
  or the tag `release-please` pushes will not trigger `release.yml`.
- **`production` GitHub Environment** (Settings → Environments) with a
  **required reviewer** — the manual approval gate for the live deploy.
- **Railway** secrets/variable on the `production` environment: `RAILWAY_TOKEN`
  (project token), `RAILWAY_SERVICE_ID`, `RAILWAY_ENVIRONMENT_ID`, and the
  `RAILWAY_PUBLIC_DOMAIN` variable.
- **`PERSONAL_GHCR_TOKEN`** secret on the `production` environment — a classic
  GitHub PAT for the `hexsleeves` account with `read:packages` (to pull the
  org image) and `write:packages` (to publish the public mirror) scopes.
  `liatrio-forge` locks container package visibility to private at the org
  level with no per-package override, so the org image can never be made
  public; `deploy-railway` mirrors it to `ghcr.io/hexsleeves/devspace-hosted`
  instead, which must be made **public** once after its first push (Package
  settings → Danger Zone → Change visibility) so Railway can pull it without
  registry credentials.

### If a release run fails

The tag already exists, so re-running is safe and idempotent from the same tag:

1. Fix the cause (or re-run if it was transient), then re-run the failed
   `release` workflow run from the Actions UI or `gh run rerun <run-id>`.
   GoReleaser runs with `--clean` and will replace partial release assets.
2. If the release was published in a broken state, delete it
   (`gh release delete v0.1.0`) and re-run the workflow — or cut a new
   `-rc.N+1` tag if the fix required code changes.

## Consumer verification

After downloading an archive and `checksums.txt` from the release page into the
same directory:

```bash
sha256sum -c --ignore-missing checksums.txt # or: shasum -a 256 -c checksums.txt
```

Verify build provenance (proves the artifact was built by this repository's
release workflow). Note: GitHub artifact attestations are only generated while
the repository is public — the attest step skips automatically on private
repositories, so provenance verification applies to releases cut after the
repo is made public:

```bash
gh attestation verify checksums.txt --repo liatrio-forge/devdrop-capstone
gh attestation verify devspace_v0.1.0_<goos>_<goarch>.tar.gz --repo liatrio-forge/devdrop-capstone
```

Then unpack and install:

```bash
tar -xzf devspace_v0.1.0_<goos>_<goarch>.tar.gz
install -m 0755 devspace_v0.1.0_<goos>_<goarch>/devspace /usr/local/bin/devspace
```

## Install from source

Build a local binary into `bin/devspace`:

```bash
make build
```

Optionally copy it into a user-writable directory on your `PATH`:

```bash
mkdir -p "$HOME/.local/bin"
install -m 0755 bin/devspace "$HOME/.local/bin/devspace"
```

If `~/.local/bin` is not already on your `PATH`, add it in your shell profile
before running `devspace`.

Verify the installed command:

```bash
devspace --help
```

You can also run the equivalent Go command directly:

```bash
go build -trimpath -o bin/devspace ./cmd/devspace
```

## Local verification targets

CI runs the same gate on every PR, but you can run it locally:

```bash
make verify
```

`make verify` runs:

1. `go test ./...`
2. `go vet ./...`
3. `go build -trimpath -o bin/devspace ./cmd/devspace`

Individual targets are also available:

```bash
make test
make vet
make build
```

To dry-run the release build locally (requires `goreleaser`):

```bash
goreleaser check
goreleaser release --snapshot --clean
```
