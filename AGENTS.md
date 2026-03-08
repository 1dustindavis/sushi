# AGENTS.md

## Repo at a glance
- `sushi` is a Go CLI wrapper around `chef-client`/`cinc-client` for local-first convergence.
- Key references:
  - `README.md` for user-facing behavior.
  - `docs/config-reference.md`, `docs/command-reference.md`, `docs/exit-code-reference.md`, `docs/operations/runbook.md` for operator docs.

## Fast start for impactful changes
- Read these first (in order):
  1. `cmd/sushi/main.go` (CLI entry points and exit classification)
  2. `internal/config/config.go` (validation and defaults assumptions)
  3. `internal/source/resolver.go` + `internal/source/remote.go` (selection, remote caching/fallback)
  4. `internal/runtime/run.go` + `internal/runtime/discovery.go` (execution and binary discovery)
  5. `integration/cli_test.go` (end-to-end behavior expectations)
- If you change behavior, update docs and tests in the same PR.

## How to work successfully here
- Keep changes cross-platform (`linux`, `macOS`, `windows`); prefer `filepath` and avoid shell-specific assumptions in core logic.
- Keep config behavior deterministic and validated; update tests whenever validation or source selection changes.
- Preserve CLI ergonomics and clear error messages, especially around operational failures.
- Prefer small, focused PRs aligned to production hardening over broad rewrites.

## Code + test workflow
- Main entrypoint: `cmd/sushi/main.go`.
- Core packages live under `internal/` (`config`, `runtime`, `source`, `logging`).
- Integration tests live under `integration/` and are the best guardrail for command behavior.
- Run formatting and tests before committing:
  - `gofmt -w .`
  - `make test`

## Change checklist (before opening PR)
- Behavior change covered by unit and/or integration tests.
- User-visible behavior reflected in docs (`docs/*.md` + `README.md` when relevant).
- Exit codes and error classification remain intentional.
- Remote fallback, stale-cache, and lock semantics remain deterministic.
