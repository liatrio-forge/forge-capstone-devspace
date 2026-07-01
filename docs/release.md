# DevDrop release packaging

DevDrop ships the local-first CLI as a `devspace` binary built from the existing
Go module. The release workflow below only packages the CLI; it does not add
hosted sync, a daemon, FUSE behavior, managed team identity, or dependency install
behavior.

## Prerequisites

- Go toolchain matching `go.mod`.
- Git, so version metadata can be derived from tags or commits.
- `sha256sum` or `shasum`, used by `make checksums` to generate SHA256 files.

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

Run the full local packaging verification before creating release artifacts:

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

## Build release artifacts

Create a release archive for the current platform and generate SHA256 checksums:

```bash
make release
```

By default, the archive is named with the current Git description and platform:

```text
dist/devspace_<version>_<goos>_<goarch>.tar.gz
dist/SHA256SUMS
```

The archive contains:

- `devspace` binary
- `README.md`
- `RELEASE.md` copy of this release guide

To override the release version string, pass `VERSION` explicitly:

```bash
make release VERSION=v0.1.0
```

## GitHub Release assembly

1. Start from a clean working tree.
2. Run `make verify`.
3. Tag the commit when appropriate:

   ```bash
   git tag v0.1.0
   ```

4. Build the current-platform release archive:

   ```bash
   make release VERSION=v0.1.0
   ```

5. Inspect the generated files:

   ```bash
   ls -lh dist/
   cat dist/SHA256SUMS
   ```

6. Create a GitHub Release using the tag and upload:

   - `dist/devspace_v0.1.0_<goos>_<goarch>.tar.gz`
   - `dist/SHA256SUMS`

7. In the release notes, include the exact commit SHA, supported platform, and
   checksum contents. Do not include secrets or machine-local configuration.

## Consumer checksum verification

After downloading an archive and `SHA256SUMS` into the same directory, verify the
artifact before unpacking it:

```bash
sha256sum -c SHA256SUMS # or: shasum -a 256 -c SHA256SUMS
```

Then unpack and install:

```bash
tar -xzf devspace_v0.1.0_<goos>_<goarch>.tar.gz
install -m 0755 devspace_v0.1.0_<goos>_<goarch>/devspace /usr/local/bin/devspace
```
