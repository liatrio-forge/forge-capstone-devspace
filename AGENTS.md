# Repository Guidelines

## Project Structure & Module Organization

DevSpace is a Go CLI. The command entry point is `cmd/devspace/main.go`; most implementation code lives in `internal/devspace/`. Tests sit beside the code as `*_test.go` files. User-facing docs and capstone materials are under `docs/`, sample manifests are under `examples/`, release automation is in `.goreleaser.yaml`, and CI lives in `.github/workflows/`. Generated build outputs belong in `bin/` or `dist/` and should not be treated as source.

## Build, Test, and Development Commands

- `go test ./...`: run the full Go test suite.
- `go vet ./...`: run Go static checks used by CI.
- `go build -trimpath -o bin/devspace ./cmd/devspace`: build the CLI locally.
- `go run ./cmd/devspace --help`: run the CLI from source.
- `make verify`: run the local CI gate: tests, vet, lint, govulncheck, and build.
- `make clean`: remove local `bin/` and `dist/` outputs.

## Coding Style & Naming Conventions

Use standard Go formatting. Run `gofmt` on changed Go files before submitting. Keep package names short and lowercase. Name tests after the behavior being validated, such as `TestDoctorReportsMissingManifest`. Prefer explicit, user-facing command errors over silent fallbacks. Keep CLI behavior local-first unless a command explicitly opts into Git or hosted sync.

## Testing Guidelines

Use Go's built-in `testing` package. Put tests next to the implementation in `internal/devspace/` and name files `*_test.go`. Cover command behavior, filesystem edge cases, manifest sync, encryption, migration, and safety checks. Before opening a PR, run:

```bash
make verify
```

## Commit & Pull Request Guidelines

Git history uses Conventional Commit-style prefixes: `feat:`, `fix:`, `docs:`, and `refactor:`. Keep commit subjects imperative and scoped to one change. Pull requests should describe the user-visible change, link any issue or spec, list verification commands, and include screenshots or terminal output when CLI output changes. For release or CI changes, mention affected workflows and any GoReleaser impact.

## Security & Configuration Tips

Do not commit real `.env` files, hosted sync tokens, age identities, or generated workspace state from `~/.devspace/` or `.devspace/`. Use placeholders in docs and examples. Treat `devspace watch --sync hosted`, env profile commands, and manifest remote changes as security-sensitive paths that need focused tests.

## Cursor Cloud specific instructions

Standard commands live in the `Makefile` and `README.md`; use `make verify` (Go gate: test/vet/lint/govulncheck/build), `make tui-verify` (Bun typecheck + tests for `tui/`), or `make ci` for both. Non-obvious caveats:

- `go.mod` pins Go 1.26.5. The VM's base `go`/`bun` are provided via symlinks in `/usr/local/bin` (Go 1.26.5 in `/usr/local/go`, Bun in `~/.bun`); the distro's `/usr/bin/go` is an older 1.22 and must not be the default. This matters for lint: `make lint` runs golangci-lint via `go run`, which inherits the base Go version — with a Go < 1.26.5 base it fails with "the Go language version ... used to build golangci-lint is lower than the targeted Go version". If lint hits that error, ensure `go version` reports 1.26.5.
- The CLI is local-first and needs no running services for core end-to-end testing. Drive it with an isolated `DEVSPACE_HOME` (temp dir) plus a temp workspace, exactly like the Go tests do; the README "Full Local Workflow" section is a good end-to-end script (init → scan → workspace push/pull → plan → apply → status, plus `env` profiles).
- Optional subsystems: FUSE mount tests are gated behind `go test ./internal/devspace -tags fusetest` and need `fuse3` + `/dev/fuse` (not installed by default); hosted sync uses `devspace hosted serve`; the OpenTUI companion (`tui/`, Bun) is optional and not part of `make verify`.
