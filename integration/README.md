# Integration tests

This directory contains end-to-end CLI tests for `sushi`.

## Scope

The integration suite validates both local and remote source behavior for core commands:

- `sushi print-plan`
- `sushi doctor`
- `sushi run`

It verifies command output and confirms `run` invokes the configured client in local/zero mode with required flags.

## Test layout

- `cli_test.go`: integration test entrypoint.
  - `TestIntegration`: top-level suite with subtests for:
    - local-source smoke coverage
    - remote-source matrix coverage
    - lock-file behavior
- `testdata/local-cookbooks/`: minimal cookbook tree used by local-source tests.
- `testdata/fakeclient/`: tiny fake client binary used to capture invocation arguments.

## Remote matrix coverage

The remote integration matrix validates:

- good and bad checksum behavior
- good and bad URL behavior
- success/failure combinations for `allow_insecure` and `skip_checksum`
- supported compression extensions (`.tar.gz`, `.tgz`, `.tar.zst`, `.tar.rst`)

The remote tests use an in-test HTTP server that serves bundles and checksum responses.

## Lock-file coverage

Integration tests exercise both lock states:

- success when `execution.lock_file` is configured and no lock file exists
- failure when the lock file already exists

## How it works

1. Build the fake client binary for the current OS.
2. Generate temporary Sushi config files per test scenario.
3. Execute CLI commands through `go run ./cmd/sushi ...`.
4. Assert command output and captured fake-client arguments.

## Running locally

From repository root:

```bash
go test ./integration -run TestIntegration -v
```

Or run the full suite:

```bash
go test ./...
```

## CI

Integration tests run in GitHub Actions via:

- `.github/workflows/integration.yml`

The workflow runs on a multi-OS matrix for every push.
