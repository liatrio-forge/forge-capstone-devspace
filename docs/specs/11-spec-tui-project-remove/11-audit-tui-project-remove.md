# 11-audit-tui-project-remove.md

## Executive Summary

- Overall Status: PASS
- Required Gate Failures: 0
- Flagged Risks: 0

## Gateboard

| Gate | Status | Why it failed (<=10 words) | Exact fix target |
| --- | --- | --- | --- |
| Requirement-to-test traceability | PASS | - | - |
| Proof artifact verifiability | PASS | - | - |
| Repository standards consistency | PASS | - | - |
| Open question resolution | PASS | - | - |
| Regression-risk blind spots | PASS | - | - |
| Non-goal leakage | PASS | - | - |

## Standards Evidence Table

| Source File | Read | Standards Extracted | Conflicts |
| --- | --- | --- | --- |
| `AGENTS.md` | yes | Go CLI entrypoint is `cmd/devspace/main.go`; implementation belongs in `internal/devspace/`; tests sit beside code; run `make verify`; do not commit secrets or generated workspace state. | none |
| `README.md` | yes | `devspace ui` is local-first; existing TUI actions are scan, plan, apply-safe, and hydrate; JSON output must be clean; scan refreshes saved project metadata. | none |
| `CONTRIBUTING.md` | not found | n/a | none |
| `.github/pull_request_template.md` | not found | n/a | none |
| `Makefile` | yes | `make verify` runs Go tests, vet, lint, vulncheck, build; `make tui-verify` runs Bun typecheck and tests; `make ci` runs both Go and TUI gates. | none |
| `tui/package.json` | yes | TUI scripts are `bun run typecheck`, `bun test`, and `bun run build`; implementation uses TypeScript/OpenTUI/React. | none |
| `.github/workflows/ci.yml` | yes | CI runs `make verify`, `make tui-verify`, and FUSE integration separately. | none |
