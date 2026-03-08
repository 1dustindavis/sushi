# AGENTS.md

## Repo at a glance
- `sushi` is a Go CLI wrapper around `chef-client`/`cinc-client` for local-first convergence.
- Current state is early scaffolding (Phase 0): config validation, source resolution, `doctor`, `print-plan`, logging, and CI.
- Key references: `README.md` for user-facing behavior and `docs/PLAN.md` for roadmap and design intent.

## How to work successfully here
- Keep changes cross-platform (`linux`, `macOS`, `windows`); prefer `filepath` and avoid shell-specific assumptions in core logic.
- Keep config behavior deterministic and validated; update tests when validation or source selection changes.
- Prefer small, focused PRs that align with the phased plan instead of broad rewrites.
- Preserve CLI ergonomics (`run`, `doctor`, `print-plan`) and clear error messages.

## Code + test workflow
- Main entrypoint: `cmd/sushi/main.go`.
- Core packages live under `internal/` (`config`, `runtime`, `source`, `logging`).
- Run formatting and tests before committing:
  - `gofmt -w .`
  - `make test`
  - `go test ./...` (optional quick rerun for focused iteration)
- Prefer `make test` before committing so race and coverage checks run consistently.
- CI runs test suites with `go test -race -cover` on all three OS targets; avoid introducing OS-specific breakage.
