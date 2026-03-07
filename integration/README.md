# Integration tests

This directory contains end-to-end CLI smoke tests for `sushi`.

## Scope

The integration suite currently focuses on **local-source** behavior and validates that the primary commands work with a generated test config:

- `sushi print-plan`
- `sushi doctor`
- `sushi run`

The tests verify expected command output and ensure `run` invokes the configured client in local/zero mode with required flags.

## Test layout

- `cli_test.go`: Main integration test entrypoint.
- `testdata/local-cookbooks/`: Minimal cookbook tree used by local-source tests.
- `testdata/fakeclient/`: Tiny fake client binary used to capture invocation arguments.

## How it works

1. Build the fake client binary for the current OS.
2. Generate a temporary local-only Sushi config.
3. Execute CLI commands through `go run ./cmd/sushi ...`.
4. Assert command output and captured fake-client arguments.

## Running locally

From repository root:

```bash
go test ./integration -v
```

Or run the full suite:

```bash
go test ./...
```

## CI

Integration tests are run in GitHub Actions via `.github/workflows/integration.yml` on a multi-OS matrix.
