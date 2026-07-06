# macOS Local Mount Proof

This worksheet records the evidence from a local macOS FUSE mount smoke test run. When complete, it closes only the PENDING local-proof marker in `docs/architecture/fuse-lazy-mount.md`, confirming that `devspace mount` operates correctly on macOS with macFUSE installed. The spec-02 Plan 015 validation gap stays separate because it requires GitHub Actions FUSE probe evidence.

Status: COMPLETE 2026-07-06

## Build

Build the binary first; all later steps run `./bin/devspace`:

```bash
mkdir -p bin
go build -trimpath -o bin/devspace ./cmd/devspace
```

## macOS Version and Build

Run:

```bash
sw_vers
```

Output:

```text
ProductName:    macOS
ProductVersion: 26.6
BuildVersion:   25G5052e
```

## macFUSE Readiness Check

Run:

```bash
test -d /Library/Filesystems/macfuse.fs && echo "macFUSE present"
test -x /Library/Filesystems/macfuse.fs/Contents/Resources/mount_macfuse && echo "mount_macfuse present"
```

Output:

```text
macFUSE present
mount_macfuse present
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
./bin/devspace init --workspace ~/Projects/<workspace>
./bin/devspace project add <project-slug>
```

Then run the preview:

```bash
rm -rf /tmp/devspace-mount-smoke
mkdir -p /tmp/devspace-mount-smoke
./bin/devspace mount /tmp/devspace-mount-smoke --preview
```

Output:

```text
DevSpace lazy mount preview
FUSE library: github.com/hanwen/go-fuse/v2/fs

┌───────────┬───────┬──────────────┬────────┬───────┬───────┬────────┐
│ PATH      │ TYPE  │ HYDRATE MODE │ STATUS │ DIRTY │ ENV   │ REASON │
├───────────┼───────┼──────────────┼────────┼───────┼───────┼────────┤
│ apps/demo │ local │ manual       │ local  │ false │ false │        │
└───────────┴───────┴──────────────┴────────┴───────┴───────┴────────┘
```

## Real Mount: Attachment and File Access

In Terminal 1, start the mount:

```bash
PATH="/Library/Filesystems/macfuse.fs/Contents/Resources:$PATH" \
  ./bin/devspace mount /tmp/devspace-mount-smoke
```

Terminal 1 Output (should show mount starting):

```text
time="2026/07/06 15:44:29" level=info msg="mounted workspace" mountpoint=/tmp/devspace-mount-smoke
time="2026/07/06 15:44:29" level=info msg="accessing an on-demand Git project through the mount triggers `devspace project hydrate` safety checks"
time="2026/07/06 15:44:29" level=info msg="press ctrl-c to unmount"
```

In Terminal 2, verify the mount and read files:

```bash
mount | grep devspace-mount-smoke
ls -la /tmp/devspace-mount-smoke
cat /tmp/devspace-mount-smoke/apps/demo/README.md
```

Terminal 2 Output:

```text
/var/folders/cn/2ck5t0pd23gc7324cjxtc76m0000gn/T/tmp.aE1cMEo875/workspace on /private/tmp/devspace-mount-smoke (macfuse, nodev, nosuid, read-only, synchronous, mounted by lecoqjacob)

total 0
drwxr-xr-x    0 root  wheel      0 Dec 31  1969 .
drwxrwxrwt  320 root  wheel  10240 Jul  6 15:45 ..
drwxr-xr-x    0 root  wheel      0 Dec 31  1969 apps

hello from devspace
```

## Unmount

In Terminal 1, press `Ctrl-C` to stop the mount process.

Then verify detachment:

```bash
mount | grep devspace-mount-smoke || echo "unmounted"
```

Unmount Output:

```text
unmounted
```

Sent `SIGINT` (`kill -INT <pid>`) to the foreground mount process instead of an
interactive `Ctrl-C` (run non-interactively via an agent shell); the mount
process handled it identically and exited cleanly. `pgrep -af` briefly
matched the exiting PID immediately after the signal, but `ps -p <pid>`
confirmed the process was already gone — no stuck process, no `kill -9`
needed.

## Troubleshooting Observed

None. The run followed the playbook exactly with no deviations beyond the
`SIGINT` note above.

## After the Run

1. Flip the PENDING local-proof marker in `docs/architecture/fuse-lazy-mount.md` to reference this proof file.
2. Leave the Plan 015 validation gap in `docs/specs/02-spec-hardening-plan-execution/02-validation-hardening-plan-execution.md` unchanged unless separate GitHub Actions FUSE probe evidence is available.
