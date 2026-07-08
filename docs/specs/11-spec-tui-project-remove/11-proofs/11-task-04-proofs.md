# Task 04 Proofs - Demo And Verification Evidence

## Task Summary

This task captures the end-to-end TUI remove demo and repository verification evidence.

## Evidence Summary

- `README.md` documents TUI project untracking and states files on disk are not touched.
- VHS demo script uses temporary `DEVSPACE_HOME` and workspace data only.
- Full TUI and Go verification gates were run after implementation.

## Artifact: VHS tape

**Path:**

```text
docs/specs/11-spec-tui-project-remove/11-tui-project-remove.tape
```

## Artifact: VHS GIF

**Path:**

```text
docs/specs/11-spec-tui-project-remove/11-tui-project-remove.gif
```

## Artifact: verification

Commands and output are recorded after the final verification run.

**Command:**

```bash
vhs docs/specs/11-spec-tui-project-remove/11-tui-project-remove.tape
```

**Result summary:** Rendered a 1000 x 700 GIF proof artifact.

```text
Creating docs/specs/11-spec-tui-project-remove/11-tui-project-remove.gif...
docs/specs/11-spec-tui-project-remove/11-tui-project-remove.gif: GIF image data, version 89a, 1000 x 700
```

**Command:**

```bash
GOCACHE=$(mktemp -d) go test ./internal/devspace -run 'TestUIServer.*Remove|TestAccessRoleAdvisory|TestRemoveProject' -v
```

**Result summary:** Targeted remove/advisory tests passed.

```text
PASS
ok  	github.com/liatrio-forge/devdrop-capstone/internal/devspace	0.963s
```

**Command:**

```bash
make tui-verify
```

**Result summary:** TUI install, typecheck, and tests passed.

```text
45 pass
0 fail
101 expect() calls
Ran 45 tests across 6 files.
```

**Command:**

```bash
GOCACHE=$(mktemp -d) make verify
```

**Result summary:** Go test, vet, gofmt check, golangci-lint, govulncheck, and build passed.

```text
go test ./...
ok  	github.com/liatrio-forge/devdrop-capstone/internal/devspace	34.347s
go vet ./...
test -z "$(gofmt -l cmd internal)" || (gofmt -l cmd internal && exit 1)
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...
0 issues.
go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
No vulnerabilities found.
mkdir -p bin
go build -trimpath -o bin/devspace ./cmd/devspace
```
