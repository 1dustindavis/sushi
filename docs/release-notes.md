# Release Notes: Current Features and Capabilities

## Product scope

- `sushi` is a CLI wrapper for `chef-client`/`cinc-client` focused on local-first convergence without requiring Chef Server.
- Supported source types: `local`, `remote`, and `chef_server`.
- Source selection is driven by ordered `source_order` configuration.

## Command surface

- `sushi run -config <path>` executes source resolution and converge.
- `sushi -config <path>` is an alias for `sushi run`.
- `sushi fetch -config <path>` fetches/verifies/activates remote bundle without converge.
- `sushi doctor -config <path>` validates configuration, dependency discovery, and source resolution.
- `sushi print-plan -config <path>` prints deterministic source-selection decisions.
- `sushi archive [<cookbook_dir>] [-o|--output <archive_path>] [--checksum]` creates remote-compatible cookbook archives.
- `sushi version` prints build version.
- Windows-only service commands: `sushi service run`, `install`, `uninstall`, `start`, `stop`, `status`.

## Configuration capabilities

- Client binary mode supports auto-discovery (`cinc-client` then `chef-client`) or explicit binary selection.
- Source enablement and ordered preference are configurable and validated.
- Local source supports configurable cookbook path.
- Remote source supports URL, checksum URL, TLS policy (`allow_insecure`), checksum enforcement (`require_checksum`), cache directory, refresh interval, request timeout, retries, and retry backoff.
- Remote fetch supports conditional requests using `ETag`/`Last-Modified` with `304 Not Modified` handling.
- Remote cache policy supports max cache age, stale warning window, cached fallback, and stale-failure enforcement.
- Chef Server source supports `client.rb` path and optional health check endpoint with configurable timeout.
- Execution settings support run list file, JSON attributes file, lock file, lock wait timeout, lock poll interval, lock stale age, and converge timeout.

## Platform and runtime behavior

- Platform-aware config path precedence is supported for Linux, macOS, and Windows.
- Environment overrides are supported for config path (`SUSHI_CONFIG_PATH`) and log path (`SUSHI_LOG_PATH`).
- Default log paths are platform-specific.
- Missing log or remote cache directories are created at runtime with warnings.
- Linux/macOS scheduled execution is supported via external schedulers (for example `systemd` timer or `launchd`).

## Operational behaviors

- Converge failures that match retryable exception patterns can trigger fallback attempts to the next usable source.
- Retryable match patterns include: `connection refused`, `connection reset by peer`, `timeout`, `temporarily unavailable`, and `503`.
- `fetch` supports metadata refresh without full download when upstream returns `HTTP 304`.
- Structured diagnostic fields include `selected_mode`, `candidate_count`, `attempt_index`, `converge_latency_ms`, `retryable_failure`, and `bundle_digest`.

## Exit code classification

- `10`: configuration invalid.
- `11`: dependency missing.
- `12`: source unavailable.
- `13`: stale cache policy violation.
- `14`: converge failure.
- `1`: unknown operational error.
