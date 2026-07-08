# Plan 021: Make release-check build devspace-tui assets before the GoReleaser dry-run

> **Executor instructions**: Follow this plan step by step. Run every verification command and confirm the expected result before moving to the next step. If anything in the "STOP conditions" section occurs, stop and report — do not improvise. When done, update the status row for this plan in `plans/README.md`.
>
> **Drift check (run first)**: `git diff --stat cedcbc7..HEAD -- .github/workflows/release-check.yml .github/workflows/release.yml .goreleaser.yaml Makefile tui/package.json tui/build-all.sh`
> If any in-scope file changed since this plan was written, compare the "Current state" excerpts against live code before proceeding; on mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none
- **Category**: dx
- **Planned at**: commit `cedcbc7`, 2026-07-08

## Why this matters

The real release workflow builds `devspace-tui` binaries before GoReleaser attaches and checksums them. The PR release-check workflow runs GoReleaser without setting up Bun or building those files, so a release-config PR can pass without validating the TUI asset path that stable releases now depend on. This is a cheap CI alignment fix: make the dry-run prepare the same extra assets that the release job prepares.

## Current state

Relevant files:

- `.github/workflows/release.yml` sets up Bun and builds the companion before GoReleaser:

```yaml
# .github/workflows/release.yml:40-51
- uses: oven-sh/setup-bun@v2
  with:
    bun-version: 1.3.14

- name: Build devspace-tui companion binaries
  run: make tui-build-all

- name: Run GoReleaser
  uses: goreleaser/goreleaser-action@v7
  with:
    version: "~> v2"
    args: release --clean
```

- `.github/workflows/release-check.yml` does not set up Bun or build TUI assets before its dry-run:

```yaml
# .github/workflows/release-check.yml:26-36
- uses: actions/setup-go@v6
  with:
    go-version-file: go.mod
    cache: true
    cache-dependency-path: go.sum

- name: GoReleaser snapshot dry-run
  uses: goreleaser/goreleaser-action@v7
  with:
    version: "~> v2"
    args: release --snapshot --clean --skip=publish
```

- `.goreleaser.yaml` now includes the TUI binaries in checksums and release files:

```yaml
# .goreleaser.yaml:49-52,71-76
checksum:
  name_template: checksums.txt
  extra_files:
    - glob: tui/dist/devspace-tui_*

release:
  extra_files:
    - glob: tui/dist/devspace-tui_*
```

- `Makefile` already owns the release asset build:

```make
# Makefile:69-73
tui-install: ## Install devspace-tui (Bun) dependencies
	cd tui && bun install --frozen-lockfile

tui-build-all: tui-install ## Build devspace-tui for all release platforms
	cd tui && ./build-all.sh
```

- `tui/build-all.sh` emits files named `tui/dist/devspace-tui_<os>_<arch>`, matching `.goreleaser.yaml`.

## Commands you will need

| Purpose | Command | Expected on success |
|---------|---------|---------------------|
| Workflow syntax sanity | `goreleaser check` | configuration valid |
| TUI release asset build | `make tui-build-all` | creates `tui/dist/devspace-tui_linux_amd64`, `linux_arm64`, `darwin_amd64`, `darwin_arm64` |
| TUI gate | `make tui-verify` | exit 0 |

Local note: a full `goreleaser release --snapshot --clean --skip=publish` may require a running Docker daemon because this repo uses GoReleaser `ko` to build the hosted image. If Docker is unavailable locally, record that and rely on the GitHub runner for the full dry-run.

## Scope

**In scope**:

- `.github/workflows/release-check.yml`

**Out of scope**:

- `.github/workflows/release.yml` unless a line moved and the current-state excerpt is stale.
- `.goreleaser.yaml` unless GoReleaser syntax changed and the dry-run cannot use the current config.
- `Makefile`, `tui/package.json`, or `tui/build-all.sh` unless the existing target is broken.
- Any release publishing, tagging, or GitHub issue work.

## Git workflow

- Branch: `advisor/021-release-check-tui-assets`
- Commit message style: conventional commit, e.g. `ci: build tui assets in release-check`.
- Do not push or open a PR unless instructed.

## Steps

### Step 1: Mirror the release workflow's Bun setup in release-check

In `.github/workflows/release-check.yml`, insert the Bun setup immediately after `actions/setup-go@v6` and before the GoReleaser dry-run:

```yaml
- uses: oven-sh/setup-bun@v2
  with:
    bun-version: 1.3.14

- name: Build devspace-tui companion binaries
  run: make tui-build-all
```

Use the same Bun version and command as `.github/workflows/release.yml` to avoid a second source of truth.

**Verify**: `grep -n "setup-bun\|make tui-build-all\|GoReleaser snapshot" .github/workflows/release-check.yml` → the setup/build lines appear before the GoReleaser snapshot step.

### Step 2: Validate the commands locally

Run the existing local checks that do not publish anything.

**Verify**:

- `goreleaser check` → `1 configuration file(s) validated`.
- `make tui-build-all` → exits 0 and creates all four `tui/dist/devspace-tui_*` files.
- `make tui-verify` → exit 0.

Do not commit `tui/dist/`; it is ignored build output.

### Step 3: Optional full dry-run if Docker is available

If `docker info` succeeds, run:

```bash
goreleaser release --snapshot --clean --skip=publish
```

Expected: exit 0.

If Docker is unavailable, do not install Docker. Record this in the PR/summary instead:

```text
Skipped full local GoReleaser snapshot: Docker daemon unavailable; release-check runs it on GitHub Actions where Docker is available.
```

**Verify**: either the snapshot exits 0 or the Docker-unavailable skip is documented.

## Test plan

- The workflow diff itself is the behavior change.
- `make tui-build-all` proves the release-check job's new step can produce the files GoReleaser expects.
- `goreleaser check` proves the current `.goreleaser.yaml` remains syntactically valid.
- GitHub Actions will prove the full dry-run with Docker/ko.

## Done criteria

- [ ] `.github/workflows/release-check.yml` sets up Bun 1.3.14.
- [ ] `.github/workflows/release-check.yml` runs `make tui-build-all` before the GoReleaser snapshot dry-run.
- [ ] `goreleaser check` exits 0.
- [ ] `make tui-build-all` exits 0 locally, or the executor documents a missing Bun/toolchain STOP condition.
- [ ] `make tui-verify` exits 0.
- [ ] No generated `tui/dist/` files are committed.
- [ ] No files outside `.github/workflows/release-check.yml` are modified, except `plans/README.md` status.
- [ ] `plans/README.md` row 021 is updated when complete.

## STOP conditions

Stop and report back if:

- `make tui-build-all` cannot run with Bun 1.3.14.
- GoReleaser starts failing because missing TUI assets are treated differently than expected.
- The release workflow no longer uses `make tui-build-all`, making this plan's mirroring target stale.
- Fixing the dry-run requires changing release publishing behavior.

## Maintenance notes

Whenever release.yml gains a pre-GoReleaser build step for extra files, release-check should gain the same step. Reviewers should check that release-check remains a dry-run only; it must not publish releases, tags, images, or issues.
