# Integration tests

This directory contains end-to-end CLI smoke tests for `sushi`.

## Scope

The integration suite covers both **local-source** and **remote-source** behavior. It validates that the primary commands work with generated test config:

- `sushi print-plan`
- `sushi doctor`
- `sushi run`

The tests verify expected command output and ensure `run` invokes the configured client in local/zero mode with required flags.

## Test layout

- `cli_test.go`: Integration test entrypoint for both modes.
  - `TestIntegrationLocal`: Local-mode integration smoke coverage.
  - `TestIntegrationRemote`: Remote-mode integration smoke coverage using an in-test HTTP bundle server.
- `testdata/local-cookbooks/`: Minimal cookbook tree used by local-source tests.
- `testdata/fakeclient/`: Tiny fake client binary used to capture invocation arguments.

## How it works

1. Build the fake client binary for the current OS.
2. Generate temporary Sushi config files for each test mode.
3. Execute CLI commands through `go run ./cmd/sushi ...`.
4. Assert command output and captured fake-client arguments.

For remote mode, the test also:

1. Builds an in-memory `tar.gz` bundle with a `cookbooks/` tree.
2. Serves it through a local test HTTP server.
3. Configures Sushi remote source to fetch and cache the bundle.

## Running locally

From repository root:

```bash
go test ./integration -v
```

Or run one mode explicitly:

```bash
go test ./integration -run TestIntegrationLocal -v
go test ./integration -run TestIntegrationRemote -v
```

Or run the full suite:

```bash
go test ./...
```

## CI

Integration tests are run in GitHub Actions via:

- `.github/workflows/integration-local.yml`
- `.github/workflows/integration-remote.yml`

Both run on a multi-OS matrix for every push.
