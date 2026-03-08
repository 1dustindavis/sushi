# Operational Runbook

## Commands

- Foreground converge once: `sushi run -config <path>`
- Bare invocation alias: `sushi -config <path>` (equivalent to `sushi run`)
- Prefetch remote bundle only: `sushi fetch -config <path>`
- Windows Service host: `sushi service run -config <path>`
- Windows Service management: `sushi service <install|uninstall|start|stop|status> [-config <path>]`

Linux/macOS should use the platform scheduler (`systemd` timer or `launchd StartInterval`) to run `sushi run` every 10 minutes.

## Retryable Converge Fallback Behavior

When converge fails, sushi captures client output and checks for retryable exceptions:

- `connection refused`
- `connection reset by peer`
- `timeout`
- `temporarily unavailable`
- `503`

If a match exists, sushi attempts the next usable source in `source_order`.

## Diagnostics

Sushi emits structured fields for operations:

- `selected_mode`
- `candidate_count`
- `attempt_index`
- `converge_latency_ms`
- `retryable_failure`
- `bundle_digest`

## Exit codes

- `10`: config invalid (read/parse/validation failures)
- `11`: dependency missing (client binary discovery failures)
- `12`: source unavailable (no usable configured source / remote unavailable)
- `13`: stale cache policy violation
- `14`: converge failure
