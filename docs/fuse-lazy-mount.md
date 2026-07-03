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

## CI Feasibility

Current status: **UNKNOWN** for hosted CI. No FUSE probe workflow run has been
observed, so there is no GitHub Actions evidence that hosted runners can run the
real mount path.

| Platform | Status | Evidence |
|----------|--------|----------|
| Linux `ubuntu-latest` | UNKNOWN | Normal `ci` runs `go test ./...`, `go vet ./...`, lint, vulncheck, and build without FUSE. That keeps `make verify` and CI FUSE-free, but it does not prove `/dev/fuse`, `fusermount3`, or `go-fuse/v2` mounting works on hosted runners. |
| macOS hosted runners | UNKNOWN | No macOS FUSE probe has been run. macOS support still depends on macFUSE or a compatible implementation being installed and approved by the OS. |

Blocker: Phase A requires triggering and observing a GitHub Actions probe that
checks `/dev/fuse`, `fusermount3 --version`, mounts a minimal `go-fuse/v2`
filesystem, lists it, and unmounts it. No such run was available to observe, and
no temporary probe workflow is left in the repository.

Future options:

- Add a temporary `workflow_dispatch` probe on a branch, run it on
  `ubuntu-latest`, capture the runner image and mount/unmount output, then delete
  the probe workflow before merging.
- If hosted Linux is NO-GO, evaluate a self-hosted Linux runner with FUSE
  enabled.
- If a containerized runner is used later, evaluate `--device /dev/fuse` and
  `--cap-add SYS_ADMIN` in an isolated CI environment.
- Keep the normal `verify` job FUSE-free regardless of the probe result.

## Follow-Up Cards

- Add an integration test job that runs only on FUSE-capable hosts and exercises
  actual mount traversal, hydration success, and hydration failure propagation.
- Add a richer project status view in the mount for dirty repositories, missing
  `.env` files, and setup hints.
- Decide whether the long-term mount should expose project paths, project names,
  or both through separate virtual directories.
- Add unmount diagnostics and stale mount cleanup guidance after real-user
  testing on macOS and Linux.
