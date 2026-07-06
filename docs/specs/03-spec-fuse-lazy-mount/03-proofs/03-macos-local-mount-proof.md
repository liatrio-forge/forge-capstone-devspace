# macOS Local Mount Proof

This worksheet records the evidence from a local macOS FUSE mount smoke test run. When complete, it closes the PENDING local-proof marker in `docs/architecture/fuse-lazy-mount.md` and the spec-02 Plan 015 validation gap, confirming that `devspace mount` operates correctly on macOS with macFUSE installed.

Status: AWAITING RUN

## macOS Version and Build

Run:

```bash
sw_vers
```

Output:

```bash

```

## macFUSE Readiness Check

Run:

```bash
test -d /Library/Filesystems/macfuse.fs && echo "macFUSE present"
test -x /Library/Filesystems/macfuse.fs/Contents/Resources/mount_macfuse && echo "mount_macfuse present"
```

Output:

```bash

```

## FUSE-Free Preview

Set up a test workspace with a tracked project (choose one):

### Option 1: Disposable smoke test with actual files

```bash
tmp="$(mktemp -d)"
export DEVSPACE_HOME="$tmp/home"
workspace="$tmp/workspace"

mkdir -p "$workspace/apps/demo"
printf "hello from devspace\n" > "$workspace/apps/demo/README.md"

./bin/devspace init --workspace "$workspace"
./bin/devspace project add apps/demo
```

### Option 2: Real checkout

```bash
./bin/devspace init --workspace ~/Projects/personal
./bin/devspace project add devdrop
```

Then run the preview:

```bash
rm -rf /tmp/devspace-mount-smoke
mkdir -p /tmp/devspace-mount-smoke
./bin/devspace mount /tmp/devspace-mount-smoke --preview
```

Output:

```bash

```

## Real Mount: Attachment and File Access

Build the binary:

```bash
go build -trimpath -o bin/devspace ./cmd/devspace
```

In Terminal 1, start the mount:

```bash
PATH="/Library/Filesystems/macfuse.fs/Contents/Resources:$PATH" \
  ./bin/devspace mount /tmp/devspace-mount-smoke
```

Terminal 1 Output (should show mount starting):

```bash

```

In Terminal 2, verify the mount and read files:

```bash
mount | grep devspace-mount-smoke
ls -la /tmp/devspace-mount-smoke
cat /tmp/devspace-mount-smoke/apps/demo/README.md
```

Terminal 2 Output:

```bash

```

## Unmount

In Terminal 1, press `Ctrl-C` to stop the mount process.

Then verify detachment:

```bash
mount | grep devspace-mount-smoke || echo "unmounted"
```

Unmount Output:

```bash

```

## Troubleshooting Observed

(Optional; record any issues encountered and how they were resolved.)

## After the Run

1. Flip the PENDING local-proof marker in `docs/architecture/fuse-lazy-mount.md` to reference this proof file.
2. Record Plan 015 closure in `docs/specs/02-spec-hardening-plan-execution/02-validation-hardening-plan-execution.md` (currently "PASS WITH GAP") referencing this proof file.
