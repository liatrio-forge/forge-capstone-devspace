# DevDrop release packaging

DevDrop ships the local-first CLI as a `devspace` binary built from the existing
Go module. Releases are **automated with GoReleaser**: pushing a version tag
builds multi-platform archives and publishes them to GitHub Releases with
checksums, a generated changelog, and build-provenance attestation. The manual
`make release` flow remains available as an offline fallback. This workflow only
packages the CLI; it does not add hosted sync, a daemon, FUSE behavior, managed
team identity, or dependency install behavior.

## Automated release (primary)

### How it works

- `.github/workflows/ci.yml` runs `go test`, `go vet`, and a build on every PR
  and push to `main`.
- `.github/workflows/release-check.yml` runs a GoReleaser snapshot dry-run on
  PRs that touch `.goreleaser.yaml` or the release workflows.
- `.github/workflows/release.yml` runs GoReleaser when a `v*` tag is pushed and
  then attests the artifacts with GitHub artifact attestations.

Each release contains four tar.gz archives (`linux`/`darwin` × `amd64`/`arm64`)
named `devspace_<version>_<os>_<arch>.tar.gz`, plus a `checksums.txt` file
(SHA256). Windows is not built: the `go-fuse` dependency does not compile on
Windows. Tags with a prerelease suffix (for example `v0.1.0-rc.1`) are marked
as GitHub prereleases automatically.

### Cutting a release

1. Make sure `main` is green (the `ci` workflow passed on the release commit).
2. Tag and push:

   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

3. Watch the `release` workflow, then verify the published release:

   ```bash
   gh run watch --exit-status "$(gh run list --workflow release --limit 1 --json databaseId --jq '.[0].databaseId')"
   gh release view v0.1.0
   ```

No manual uploads are needed; GoReleaser creates the GitHub Release, attaches
the archives and `checksums.txt`, and generates the changelog from
conventional-commit history (`feat:`/`fix:` grouped, `docs:`/`chore:`/`test:`
excluded).

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
gh attestation verify checksums.txt --repo HexSleeves/devdrop
gh attestation verify devspace_v0.1.0_<goos>_<goarch>.tar.gz --repo HexSleeves/devdrop
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
go build -trimpath -o bin/devspace ./cmd/devdrop
```

## Local verification targets

CI runs the same gate on every PR, but you can run it locally:

```bash
make verify
```

`make verify` runs:

1. `go test ./...`
2. `go vet ./...`
3. `go build -trimpath -o bin/devspace ./cmd/devdrop`

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

## Manual release (offline fallback)

Use this only when the automated pipeline is unavailable. It produces a
current-platform archive plus `dist/SHA256SUMS` (note: the automated pipeline
names its checksum file `checksums.txt` instead).

1. Start from a clean working tree and run `make verify`.
2. Build the archive, overriding the version to match the tag:

   ```bash
   make release VERSION=v0.1.0
   ```

3. Inspect the generated files:

   ```bash
   ls -lh dist/
   cat dist/SHA256SUMS
   ```

4. Create a GitHub Release using the tag and upload:

   - `dist/devspace_v0.1.0_<goos>_<goarch>.tar.gz`
   - `dist/SHA256SUMS`

5. In the release notes, include the exact commit SHA, supported platform, and
   checksum contents. Do not include secrets or machine-local configuration.
