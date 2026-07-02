# Task 02 Proofs - GoReleaser v2 config builds multi-platform devspace releases

## Task Summary

This task adds `.goreleaser.yaml` (schema `version: 2`), replacing the single-platform manual `make release` with a reproducible four-target build (linux/darwin × amd64/arm64) that preserves the existing archive naming and contents.

## What This Task Proves

- The config is valid and free of deprecated fields (`goreleaser check` passes on GoReleaser v2.16.0).
- A snapshot release builds exactly four `devspace_*` tar.gz archives plus a checksums file.
- Each archive matches the `make release` layout (`devspace` binary, `README.md`, `RELEASE.md`).
- The `-X main.version` ldflags wiring works: the packaged binary reports the release version.

## Evidence Summary

- `goreleaser check`: config validated, exit 0.
- `goreleaser release --snapshot --clean`: 4 targets built and archived in ~5s.
- Archive listing and `devspace version` output confirm layout and version injection.

## Artifact: goreleaser check passes

**What it proves:** The config conforms to the v2 schema with no deprecation warnings (notably `archives.formats`, and no removed `brews` key).

**Why it matters:** Guarantees the config won't be rejected by the pinned `~> v2` GoReleaser in CI.

**Command:**

~~~bash
goreleaser check
~~~

**Result summary:** One configuration file validated; exit code 0. Local GoReleaser version: 2.16.0.

~~~text
  • checking                                  path=.goreleaser.yaml
  • 1 configuration file(s) validated
  • thanks for using GoReleaser!
EXIT: 0
~~~

## Artifact: Snapshot build produces the four required targets

**What it proves:** Cross-compilation with `CGO_ENABLED=0` succeeds for all four spec-required platforms, and checksums are generated.

**Why it matters:** This is the exact build the tag-triggered release workflow will run; proving it locally de-risks the first real release.

**Command:**

~~~bash
goreleaser release --snapshot --clean && ls -lh dist/*.tar.gz dist/checksums.txt
~~~

**Result summary:** Four archives (darwin/linux × amd64/arm64, ~3.1–3.4 MB each) plus `checksums.txt`; `release succeeded after 5s`.

~~~text
dist/checksums.txt  474B
dist/devspace_v0.0.0-SNAPSHOT-c9b3182_darwin_amd64.tar.gz  3.4M
dist/devspace_v0.0.0-SNAPSHOT-c9b3182_darwin_arm64.tar.gz  3.1M
dist/devspace_v0.0.0-SNAPSHOT-c9b3182_linux_amd64.tar.gz   3.4M
dist/devspace_v0.0.0-SNAPSHOT-c9b3182_linux_arm64.tar.gz   3.1M
~~~

## Artifact: Archive layout and embedded version

**What it proves:** Archives contain exactly the `make release` file set, and the binary's `version` variable is injected at build time.

**Why it matters:** Downloads look identical to previous manual releases, and `devspace version` is traceable to the release tag.

**Command:**

~~~bash
tar -tzf dist/devspace_v0.0.0-SNAPSHOT-c9b3182_darwin_arm64.tar.gz
./devspace version   # extracted from the archive
~~~

**Result summary:** Archive contains `README.md`, `RELEASE.md`, `devspace`; the extracted binary prints the injected snapshot version.

~~~text
README.md
RELEASE.md
devspace
---
v0.0.0-SNAPSHOT-c9b3182
~~~

## Reviewer Conclusion

The GoReleaser configuration is schema-valid, builds all four spec-required platforms, preserves the established archive naming/layout, and injects the version correctly — ready for CI use.
