# Task 04 Proofs - UI Release Packaging

Status: DONE

## Command surface and lookup

RED:

```text
go test ./internal/devspace -run 'TestFindTUIBinary|TestUICommandDocuments|TestReleaseCommandTreeContract$' -count=1
internal/devspace/ui_server_test.go: undefined: findTUIBinaryFrom
FAIL
```

GREEN:

```text
ok github.com/liatrio-forge/devdrop-capstone/internal/devspace
```

The tests require `ui` as the only visible UI command, hidden `ui-server`,
adjacent companion preference, app-home and PATH lookup, bundled/source-build
help, and `--legacy`. A source scan finds no installer implementation or
fallback recommendation to `devspace tui install`.

Sanitized help evidence:

```text
DIAGNOSTICS AND AUTOMATION:
  ui [--flags]  Open the interactive workspace dashboard

Release archives include the bundled devspace-tui companion next to devspace.
A source build also looks in $DEVSPACE_HOME/bin and on PATH; --legacy forces
the built-in dashboard.
```

## Archive validation

Before the GoReleaser archive fix, the reproducible snapshot completed but the
archive validator failed as intended:

```text
devspace_..._linux_amd64.tar.gz: missing executable devspace-tui
```

After the fix:

```text
$ scripts/verify-release-archives.sh dist
devspace_..._linux_amd64.tar.gz: devspace devspace-tui
devspace_..._linux_arm64.tar.gz: devspace devspace-tui
devspace_..._darwin_amd64.tar.gz: devspace devspace-tui
devspace_..._darwin_arm64.tar.gz: devspace devspace-tui

$ grep -c 'devspace-tui_' dist/checksums.txt
4
```

The validator extracts each archive and requires both files to have executable
mode. `scripts/verify-release-archives_test.sh` also proves a missing companion
fails validation.

## Release and smoke evidence

```text
$ goreleaser check
1 configuration file(s) validated

$ DOCKER_HOST=unix://<local-docker-socket> \
    goreleaser release --snapshot --clean --skip=publish
release succeeded

$ make tui-verify
45 pass
0 fail

$ go test ./...
ok github.com/liatrio-forge/devdrop-capstone/internal/devspace

$ make verify
0 issues.
No vulnerabilities found.
```

The default Docker socket was absent locally, so the first full snapshot
stopped only when ko tried `/var/run/docker.sock`. Re-running against the
already-running Docker context completed the full snapshot, including ko.

For the local `darwin_arm64` archive, the proof extracted both executables,
initialized an isolated workspace, removed app-home/PATH companion candidates,
and launched the extracted `devspace ui --no-watch` under a controlled TTY:

```text
adjacent companion rendered DevSpace
```

This demonstrates the released CLI selected the companion beside it. No real
workspace paths, tokens, remotes, or user project data were captured.
