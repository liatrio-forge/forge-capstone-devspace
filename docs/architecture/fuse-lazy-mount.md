# FUSE Lazy Workspace Mount Prototype

`devspace mount <mountpoint>` is a prototype for a read-only workspace view backed
by the DevDrop manifest. It is intentionally outside normal sync, plan, apply,
and hydrate workflows so the CLI still works on machines without FUSE.

## Library Selection

The prototype uses `github.com/hanwen/go-fuse/v2/fs`.

- `go-fuse/v2` is the best fit for DevDrop because it is Go-native, current, and
  ships a higher-level `fs` node API plus loopback filesystem support.
- `bazil.org/fuse` is also Go-native, but its published package is older and the
  API would require more custom filesystem plumbing for this spike.
- `github.com/jacobsa/fuse` has useful examples, but it is less aligned with a
  manifest tree that can switch from virtual entries to loopback directories.

## Behavior

The mount exposes tracked project paths from `.devspace/manifest.json`.

- `ls <mountpoint>` lists top-level manifest path segments without hydrating
  projects.
- Traversing into an on-demand Git project runs the same safety path as
  `devspace project hydrate <project>`.
- Hydration failures are returned to the filesystem caller and logged to stderr;
  the mount does not convert them into empty successful directories.
- Local-only, manual, metadata-only, or missing projects that cannot hydrate
  automatically are represented by a stub directory containing `.devspace-status`.
- The mountpoint must be empty. DevDrop refuses to mount over non-empty
  directories so local files are not hidden.

## Platform Requirements

FUSE support is optional and platform-specific.

- macOS requires macFUSE or a compatible FUSE implementation installed and
  approved by the OS.
- Linux requires `/dev/fuse` and permission to mount FUSE filesystems, often via
  `fusermount3` or equivalent distribution packaging.
- CI and normal CLI workflows do not require FUSE.

If FUSE is unavailable, use:

```bash
devspace mount /tmp/devspace-mount --preview
```

`--preview` prints the manifest-backed entries and hydration status without
mounting anything.

## macOS Developer Readiness

Current product priority: **macOS first**. DevSpace is primarily developed on
macOS, so the mount prototype should be evaluated first as a local developer
experience, not as a hosted CI feature.

The current local machine is macOS `26.6` (`25G5052e`). macFUSE is not installed:
`/Library/Filesystems/macfuse.fs` is missing and `mount_macfuse` is not on
`PATH`. No system installation or security-policy change was attempted.

Recommended local proof path after macFUSE is installed and approved:

```bash
sw_vers
test -d /Library/Filesystems/macfuse.fs && echo "macFUSE present"
command -v mount_macfuse
devspace mount /tmp/devspace-mount --preview
devspace mount /tmp/devspace-mount
```

Expected behavior:

- `--preview` works without FUSE and remains the fallback on Macs without
  macFUSE.
- A real mount requires macFUSE or a compatible FUSE implementation to be
  installed and approved by macOS.
- DevSpace does not install macFUSE, ask for elevated macOS permissions, or
  alter system extension policy automatically.

Hosted macOS CI is not the first automation target. macFUSE has historically
depended on kernel or system extension loading, which is not a reliable
assumption on ephemeral hosted runners. macOS 26's FSKit direction is worth a
future spike, but it is separate from this Plan 015 resolution.

## CI Feasibility

Current automation status: **GO** for GitHub-hosted Linux CI. Linux is the
follow-up safety net for repeatable regression coverage after the macOS-first
developer path is documented. A temporary probe workflow on PR #25 verified
`/dev/fuse`, `fusermount3`, a minimal `go-fuse/v2` loopback mount, directory
listing, file read, and clean unmount on `ubuntu-latest`.

| Platform | Status | Evidence |
|----------|--------|----------|
| Linux `ubuntu-latest` | GO | GitHub Actions run `28685409454` on PR #25 passed. Runner image: `ubuntu-24.04`, version `20260628.225.1`. The probe observed `/dev/fuse`, `fusermount3 version: 3.14.0`, mounted a `go-fuse/v2` loopback filesystem, listed `entry: hello.txt`, read `fuse ok`, and printed `unmounted`. |
| macOS local developer machines | PENDING LOCAL PROOF | Current machine is macOS `26.6` (`25G5052e`) and does not have macFUSE installed. Product direction is macOS-first, but real mount proof waits for a developer Mac with macFUSE installed and approved. |
| macOS hosted runners | DEFERRED | Hosted macOS FUSE CI is not the first target. macFUSE approval and kernel/system extension loading are not reliable assumptions on ephemeral hosted runners. |

The temporary probe workflow was removed after the Linux GO result was recorded.
Permanent coverage now lives in the `mount-integration` CI job, which keeps the
default `verify` job and `make verify` FUSE-free.

Future options:

- Run the macOS local proof after macFUSE is installed on a developer machine.
- Evaluate a self-hosted Mac runner if automated macOS FUSE regression coverage
  becomes necessary.
- Evaluate macOS 26 FSKit-backed options when the DevSpace mount dependency path
  can use them cleanly.
- Re-run a temporary probe if GitHub runner images change in a way that breaks
  `mount-integration`.
- If a containerized runner is used later, evaluate `--device /dev/fuse` and
  `--cap-add SYS_ADMIN` in an isolated CI environment.
- Keep the normal `verify` job FUSE-free regardless of the probe result.

## Follow-Up Cards

- Complete a real macOS local mount smoke test on a Mac with macFUSE installed
  and approved.
- Expand the integration job if new mount behavior is added beyond traversal,
  hydration success, and hydration failure propagation.
- Add a richer project status view in the mount for dirty repositories, missing
  `.env` files, and setup hints.
- Decide whether the long-term mount should expose project paths, project names,
  or both through separate virtual directories.
- Add unmount diagnostics and stale mount cleanup guidance after real-user
  testing on macOS and Linux.
