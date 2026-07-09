# Task 01 Proofs - Install all target-platform native deps before cross-compiling

## Task Summary

`tui/build-all.sh` now runs `bun install --frozen-lockfile --os '*' --cpu '*'`
before the cross-compile loop, forcing every target platform's
`@opentui/core-<os>-<arch>` native optional dependency into `node_modules` so
`bun build --compile --target=<foreign>` can resolve them on a single host.

## What This Task Proves

- `make tui-build-all` exits 0 and produces all four
  `tui/dist/devspace-tui_<os>_<arch>` binaries from a single (darwin-arm64)
  host.
- Each binary is a valid executable for its declared OS/arch.
- The `--frozen-lockfile` flag prevents any lockfile drift from the `--os '*'
  --cpu '*'` install.
- The host-native binary actually runs and prints usage text.
- `make tui-verify` (typecheck + tests) still passes, so the host-only build
  path is not regressed.

## Evidence Summary

- `make tui-build-all` exit 0; four binaries produced.
- `file` on all four binaries reports the correct OS/arch for each.
- `git diff --exit-code tui/bun.lock` exits 0 — no lockfile drift.
- `tui/dist/devspace-tui_darwin_arm64 --help` prints companion usage text.
- `make tui-verify`: typecheck passes, 45/45 tests pass.

## Artifact: `tui/build-all.sh` diff

**What it proves:** The fix is a single, minimal addition — one comment plus
one install line — placed before the cross-compile loop, matching the file's
existing bash style.

**Why it matters:** This is the actual code change under test; every other
artifact demonstrates its effect.

**Command:**

```bash
/usr/bin/git -C <repo-root> diff tui/build-all.sh
```

**Result summary:** Adds a 6-line block (4-line comment + 1 blank + 1
install command) immediately after `cd "$(dirname "$0")"` and before the
`targets` array / build loop.

```diff
diff --git i/tui/build-all.sh w/tui/build-all.sh
index dc02698..8a11476 100755
--- i/tui/build-all.sh
+++ w/tui/build-all.sh
@@ -5,6 +5,12 @@
 set -euo pipefail
 cd "$(dirname "$0")"
 
+# @opentui/core ships its native module as os/cpu-gated optional deps, so a
+# plain `bun install` only materializes the host's package. Force every
+# target platform's native package into node_modules (frozen, no lockfile
+# drift) so cross-compiling below can resolve them all.
+bun install --frozen-lockfile --os '*' --cpu '*'
+
 declare -A targets=(
   [linux_amd64]=bun-linux-x64
   [linux_arm64]=bun-linux-arm64
```

## Artifact: `make tui-build-all` exits 0, all four binaries produced

**What it proves:** The cross-compile resolution failure (`Could not resolve
"@opentui/core-<os>-<arch>"`) is fixed; a single darwin-arm64 host produces
all four release binaries.

**Why it matters:** This is the core bug this spec fixes — without the
all-platform install, only the host's own architecture would resolve and the
other three targets would fail to bundle.

**Command:**

```bash
rm -rf tui/dist && make tui-build-all; echo "EXIT:$?"
```

**Result summary:** Exit 0. All four `dist/devspace-tui_*` files built with
no resolution errors.

```text
cd tui && bun install --frozen-lockfile
bun install v1.3.14 (0d9b296a)

Checked 26 installs across 34 packages (no changes) [6.00ms]
cd tui && ./build-all.sh
bun install v1.3.14 (0d9b296a)

Checked 33 installs across 34 packages (no changes) [2.00ms]
building dist/devspace-tui_linux_amd64
  [22ms]  bundle  43 modules
  [56ms] compile  dist/devspace-tui_linux_amd64 bun-linux-x64-v1.3.14
building dist/devspace-tui_darwin_amd64
  [19ms]  bundle  41 modules
  [29ms] compile  dist/devspace-tui_darwin_amd64 bun-darwin-x64-v1.3.14
building dist/devspace-tui_darwin_arm64
  [18ms]  bundle  41 modules
  [66ms] compile  dist/devspace-tui_darwin_arm64
building dist/devspace-tui_linux_arm64
  [21ms]  bundle  43 modules
  [43ms] compile  dist/devspace-tui_linux_arm64 bun-linux-aarch64-v1.3.14
EXIT:0
```

Note: the second `bun install` line ("Checked 33 installs") is the new
`--os '*' --cpu '*'` install inside `build-all.sh` — it materializes 33
installs (host + all foreign-platform natives) versus the 26 from the
plain `tui-install` Makefile dependency, with zero package changes (frozen
lockfile).

## Artifact: `file` confirms each binary's architecture

**What it proves:** Each of the four binaries actually compiled for its
declared target OS/arch, not just that a file was emitted.

**Why it matters:** This is the spec's architecture-correctness success
metric (4/4 correct).

**Command:**

```bash
file tui/dist/devspace-tui_linux_amd64 tui/dist/devspace-tui_linux_arm64 \
     tui/dist/devspace-tui_darwin_amd64 tui/dist/devspace-tui_darwin_arm64
```

**Result summary:** ELF x86-64, ELF ARM aarch64, Mach-O x86_64, Mach-O
arm64 — all four match their declared target.

```text
tui/dist/devspace-tui_linux_amd64:  ELF 64-bit LSB executable, x86-64, version 1 (SYSV), dynamically linked, interpreter /lib64/ld-linux-x86-64.so.2, for GNU/Linux 3.2.0, BuildID[sha1]=a9a0d18db4f98a86ad4778800c5fa46943f81b2e, not stripped
tui/dist/devspace-tui_linux_arm64:  ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), dynamically linked, interpreter /lib/ld-linux-aarch64.so.1, for GNU/Linux 3.7.0, BuildID[sha1]=b5b74501ad8ae6855b83b56e7ec1e6ef5eae266f, not stripped
tui/dist/devspace-tui_darwin_amd64: Mach-O 64-bit executable x86_64
tui/dist/devspace-tui_darwin_arm64: Mach-O 64-bit executable arm64
```

## Artifact: `tui/bun.lock` has zero drift

**What it proves:** The `--os '*' --cpu '*'` install did not mutate the
frozen lockfile; all eight platform packages were already recorded.

**Why it matters:** A drifting lockfile would violate the spec's
lockfile-stability requirement and the `--frozen-lockfile` contract.

**Command:**

```bash
/usr/bin/git -C <repo-root> diff --exit-code tui/bun.lock; echo "LOCKFILE_DIFF_EXIT:$?"
```

**Result summary:** Exit 0 — no diff.

```text
LOCKFILE_DIFF_EXIT:0
```

## Artifact: host-native binary smoke test

**What it proves:** The darwin-arm64 output is not merely a linkable
artifact but an actually runnable binary that starts and prints help text.

**Why it matters:** This is spec Unit 2's proof requirement — build success
alone doesn't guarantee the compiled binary executes correctly.

**Command:**

```bash
tui/dist/devspace-tui_darwin_arm64 --help; echo "EXIT:$?"
```

**Result summary:** Prints companion usage text, exits 0.

```text
devspace-tui — companion dashboard for devspace

Usage: devspace-tui [flags]

Flags:
  --no-watch   disable the workspace file watcher
  -h, --help   show this help and exit

Env:
  DEVSPACE_BIN   devspace binary to spawn (default: "devspace" on PATH)
EXIT:0
```

## Artifact: `make tui-verify` still passes

**What it proves:** The change does not regress the host-only typecheck/test
path.

**Why it matters:** The fix must be additive to the build script, not a
behavior change to the normal dev/test workflow.

**Command:**

```bash
make tui-verify; echo "EXIT:$?"
```

**Result summary:** Typecheck clean; 45/45 tests pass.

```text
cd tui && bun install --frozen-lockfile
bun install v1.3.14 (0d9b296a)

Checked 26 installs across 34 packages (no changes) [1.00ms]
cd tui && bun run typecheck && bun test
$ tsc --noEmit
bun test v1.3.14 (0d9b296a)

 45 pass
 0 fail
 101 expect() calls
Ran 45 tests across 6 files. [156.00ms]
EXIT:0
```

## Reviewer Conclusion

The fix is a minimal, additive change to `tui/build-all.sh`: one
`bun install --frozen-lockfile --os '*' --cpu '*'` line with an explanatory
comment. All four proof artifacts required by spec Unit 1 and Unit 2 pass:
build exits 0 with all four binaries present, `file` confirms correct
architecture for each, the frozen lockfile has zero drift, the host-native
binary runs and prints help text, and `make tui-verify` is unaffected. Task
1.0 is complete and verified.
