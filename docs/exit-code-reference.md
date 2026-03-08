# Exit Code Reference

| Exit code | Meaning | Typical remediation |
| --- | --- | --- |
| `10` | Config invalid (read/parse/validation failures). | Fix JSON syntax, required fields, or path values; re-run `doctor`. |
| `11` | Dependency missing (client binary discovery failed). | Install/configure `cinc-client` or `chef-client`, or set `runtime.client_binary`. |
| `12` | Source unavailable (no usable configured source / remote unavailable). | Verify source enablement/order, connectivity, and remote URLs/checksum endpoints. |
| `13` | Stale cache policy violation. | Refresh remote cache or relax `fail_if_stale`/`max_cache_age` policy. |
| `14` | Converge failure. | Inspect captured client output and retryable fallback behavior. |
| `1` | Unknown operational error. | Inspect logs for unclassified failure and raise issue with reproduction steps. |
