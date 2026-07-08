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
