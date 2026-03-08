# Operational Runbook

## References

- Commands: [`docs/command-reference.md`](../command-reference.md)
- Exit codes: [`docs/exit-code-reference.md`](../exit-code-reference.md)

## Retryable Converge Fallback Behavior

When converge fails, sushi captures client output and checks for retryable exceptions:

- `connection refused`
- `connection reset by peer`
- `timeout`
- `temporarily unavailable`
- `503`

If a match exists, sushi attempts the next usable source in `source_order`.

## Troubleshooting matrix

| Symptom | Likely cause | Quick checks | Next action |
| --- | --- | --- | --- |
| `source resolution: FAIL (...)` in `doctor` | No enabled/usable source in `source_order`. | `sushi print-plan -config <path>`; confirm source reasons list. | Fix source config or enable fallback source. |
| Exit `12` from `print-plan`/`run` | Remote/service endpoint unavailable or config points to invalid source. | Check URL reachability and checksum endpoint; run `doctor`. | Restore source availability or reorder source precedence. |
| Exit `13` with stale cache message | Cache older than policy and `fail_if_stale=true`. | Inspect cache metadata timestamps and `max_cache_age`. | Run `sushi fetch` against healthy remote; adjust policy if required. |
| `run` retries to next source unexpectedly | Retryable converge failure matched exception list. | Review logs for `retryable_failure=true` and selected fallback source. | Fix upstream transient failure or tune source order. |
| `fetch` prints `HTTP 304` | Conditional request validated unchanged upstream bundle. | Confirm `ETag` / `Last-Modified` headers upstream. | No action required; metadata expiry should refresh automatically. |
| Windows `service` command unavailable | Running non-Windows binary. | `sushi help` output and host OS. | Use scheduler examples for Linux/macOS or run on Windows host. |

## Diagnostics

Sushi emits structured fields for operations:

- `selected_mode`
- `candidate_count`
- `attempt_index`
- `converge_latency_ms`
- `retryable_failure`
- `bundle_digest`
