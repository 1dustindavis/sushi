# Configuration Reference

This document is a field-by-field reference for the `sushi` JSON configuration file. Use it to understand what each config argument accepts, whether it is required, what default behavior applies when unset, and how it affects `run`, `doctor`, and `print-plan` behavior.

## runtime

**runtime.client_binary** *string* default: `"auto"` required: no  
Specifies the client executable sushi should run for convergence. Use `auto` to let sushi discover `cinc-client` first and then `chef-client` from `PATH`. You can also set an explicit binary name (for example `cinc-client` or `chef-client`) to require that executable.

## source_order

**source_order** *list of strings* default: none required: yes  
Defines the ordered source preference list. Supported values are `local`, `remote`, and `chef_server`. The list must be non-empty, contain only known source names, and have no duplicates.

## sources.local

**sources.local.enabled** *boolean* default: `false` required: no  
Enables or disables the local source.

**sources.local.cookbook_path** *string* default: none required: conditional  
Filesystem path to local cookbooks. Required when `sources.local.enabled=true` and `local` appears in `source_order`. When using local source, the administrator is responsible for ensuring cookbooks are present and kept up to date. If you want sushi to fetch and maintain cookbook bundles, use `sources.remote`.

## sources.remote

When the remote source is enabled, sushi downloads the configured cookbook bundle, validates it (when checksum settings are enabled), stores it in cache, and resolves converges from the cached/activated cookbook tree according to refresh and fallback policy.

**sources.remote.enabled** *boolean* default: `false` required: no  
Enables or disables the remote cookbook bundle source.

**sources.remote.url** *string (absolute URL)* default: none required: conditional  
URL for downloading the remote cookbook bundle. Required when `sources.remote.enabled=true` and `remote` appears in `source_order`.

**sources.remote.checksum_url** *string (absolute URL)* default: none required: conditional  
Optional checksum endpoint used to validate bundle integrity. Required when `sources.remote.require_checksum=true`. For stronger assurance, serve checksum data from infrastructure separate from the cookbook bundle host.

**sources.remote.allow_insecure** *boolean* default: `false` required: no  
Allows HTTP (non-HTTPS) URLs for `url` and `checksum_url`. Must be `true` if either endpoint uses HTTP.

**sources.remote.require_checksum** *boolean* default: `false` required: no  
Requires checksum verification and enforces presence of `checksum_url`.

**sources.remote.refresh_interval** *duration string* default: empty (always refresh) required: no  
Minimum interval before attempting a new remote fetch when cached metadata is present.

**sources.remote.request_timeout** *duration string* default: `15s` required: no  
HTTP timeout for remote bundle and checksum fetches.

**sources.remote.fetch_retries** *integer (`>=0`)* default: `0` required: no  
Number of retry attempts after the initial download attempt.

**sources.remote.retry_backoff** *duration string* default: `500ms` required: no  
Delay between retry attempts.

**sources.remote.cache_dir** *string* default: none required: conditional  
Directory used for remote bundle cache and metadata. Required when remote source is enabled.

**sources.remote.max_cache_age** *duration string* default: empty (no max age) required: no  
Maximum allowed cache age before it is treated as stale.

**sources.remote.stale_warning_window** *duration string* default: empty (disabled) required: no  
Warning window before cache expiry when stale-age metadata exists.

**sources.remote.allow_cached_fallback** *boolean* default: `false` required: no  
Permits falling back to cached content if remote refresh fails.

**sources.remote.fail_if_stale** *boolean* default: `false` required: no  
Causes resolution failure if the selected cache is stale.

## sources.chef_server

**sources.chef_server.enabled** *boolean* default: `false` required: no  
Enables or disables chef server source configuration.

**sources.chef_server.client_rb** *string* default: none required: conditional  
Path to an existing `client.rb`. Required when `sources.chef_server.enabled=true` and `chef_server` appears in `source_order`. During `run`, this path is passed directly to `chef-client`/`cinc-client` with `-c` (server mode, not `-z`).

**sources.chef_server.healthcheck.endpoint** *string* default: empty required: no  
Optional endpoint used for Chef Server health checks during source resolution. If unset, sushi treats `chef_server` as usable once `client_rb` exists.

Healthcheck endpoint requirements and behavior:
- Endpoint should be an HTTP(S) URL reachable from the node running sushi.
- Choose an endpoint that reflects Chef Server availability for the node (for example an org-scoped API path), not a generic unrelated URL.
- A response is considered healthy only when sushi receives an HTTP status in the `2xx` range.
- Any non-`2xx` status (for example `401`, `403`, `404`, `429`, `500`) is treated as unhealthy for source selection.
- Network failures (DNS, connection refused/reset, TLS handshake errors, timeout) are treated as unhealthy.
- On failure, sushi records the exact failure reason in the source decision list (`print-plan` / `doctor`) and continues evaluating the next source in `source_order`.

**sources.chef_server.healthcheck.timeout** *duration string* default: `2s` required: no  
Optional timeout for Chef Server health check requests. Must be a valid duration and greater than zero when set. When unset, sushi uses `2s`.

## execution

**execution.run_list_file** *string* default: empty required: no  
Path to run list JSON used during converge execution.

**execution.json_attributes_file** *string* default: empty (falls back to `run_list_file`) required: no  
Path to JSON attributes passed to the client.

**execution.lock_file** *string* default: empty (locking disabled) required: no  
Lock file path used to prevent overlapping converges.

**execution.lock_wait_timeout** *duration string* default: empty required: no  
Maximum wait time while attempting to acquire the lock.

**execution.lock_poll_interval** *duration string* default: empty required: no  
Polling interval used when waiting on lock acquisition.

**execution.lock_stale_age** *duration string* default: empty required: no  
Age threshold for treating an existing lock as stale.

**execution.converge_timeout** *duration string* default: empty (no timeout) required: no  
Maximum runtime for converge execution before cancellation.


## Platform defaults

If `-config` is omitted, sushi uses a platform default path:

- Linux: `/etc/sushi/config.json`
- macOS: `/Library/Application Support/sushi/config.json`
- Windows: `%ProgramData%\sushi\config.json`

Default log file locations:

- Linux: `/var/log/sushi/sushi.log`
- macOS: `/Library/Logs/sushi/sushi.log`
- Windows: `%ProgramData%\sushi\logs\sushi.log`

Environment overrides:

- `SUSHI_CONFIG_PATH`
- `SUSHI_LOG_PATH`
