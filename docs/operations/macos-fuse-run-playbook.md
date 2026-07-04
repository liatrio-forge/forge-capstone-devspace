# macOS FUSE Run Playbook

Use this playbook to smoke-test `devspace mount` on a developer Mac after
macFUSE has been installed and approved by macOS.

## Preconditions

- You are on macOS.
- macFUSE is installed and approved in System Settings.
- The Mac has been restarted after the macFUSE approval flow.
- You are in a clean DevDrop checkout.
- The mountpoint is empty and disposable.

DevSpace does not install macFUSE, request elevated permissions, or change macOS
system extension policy.

## Build

```bash
go build -trimpath -o bin/devspace ./cmd/devspace
```

## Readiness Check

```bash
sw_vers
test -d /Library/Filesystems/macfuse.fs && echo "macFUSE present"
test -x /Library/Filesystems/macfuse.fs/Contents/Resources/mount_macfuse && echo "mount_macfuse present"
```

If `macFUSE present` prints but the real mount does not attach, restart the Mac
and run this playbook again. A restart is expected after first approving macFUSE.

## FUSE-Free Preview

```bash
rm -rf /tmp/devspace-mount-smoke
mkdir -p /tmp/devspace-mount-smoke
./bin/devspace mount /tmp/devspace-mount-smoke --preview
```

Expected result:

- The command exits successfully.
- It prints projected manifest entries, or `(no tracked projects)` for an empty
  workspace.
- It does not require FUSE.

## Real Mount Smoke Test

Run the mount in one terminal:

```bash
PATH="/Library/Filesystems/macfuse.fs/Contents/Resources:$PATH" \
  ./bin/devspace mount /tmp/devspace-mount-smoke
```

In a second terminal, verify the mount:

```bash
mount | grep devspace-mount-smoke
ls -la /tmp/devspace-mount-smoke
```

Expected result:

- The first terminal keeps running while the filesystem is mounted.
- `mount` shows `/tmp/devspace-mount-smoke`.
- `ls` succeeds and shows manifest-backed entries, or an empty root for a
  workspace with no tracked projects.

## Unmount

Stop the foreground mount with `Ctrl-C`. Then verify it detached:

```bash
mount | grep devspace-mount-smoke || echo "unmounted"
```

If the process is still running but no mount is attached:

```bash
pgrep -af "devspace mount /tmp/devspace-mount-smoke"
kill <pid>
```

Use `kill -9 <pid>` only for a stuck local smoke-test process that does not exit
after a normal signal.

## Evidence To Record

Record these fields in the SDD proof artifact or PR comment:

- macOS version and build from `sw_vers`.
- macFUSE readiness check result.
- Preview output.
- Whether the real mount attached.
- `mount` and `ls` output while mounted.
- Unmount result.

## Failure Notes

| Symptom | Likely Cause | Action |
|---------|--------------|--------|
| `macFUSE present`, but no mount attaches | macFUSE was installed but not active yet | Restart macOS, then rerun. |
| `mount_macfuse` not on `PATH` | macFUSE resources are not exported through the shell path | Prefix `PATH` with `/Library/Filesystems/macfuse.fs/Contents/Resources`. |
| `--preview` works but real mount fails | FUSE-specific runtime issue | Keep normal CLI workflows unblocked and capture stderr. |
| mountpoint has files | DevSpace refuses to hide local files | Use a fresh empty directory. |
