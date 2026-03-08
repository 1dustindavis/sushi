# sushi product + implementation plan

## 1) Product intent

Build a robust command wrapper for `chef-client`/`cinc-client` that:

1. Assumes **no Chef Server** by default.
2. Supports **cookbooks from local disk** or a **remote compressed bundle URL**.
3. Optionally supports Chef Server mode for organizations that still use it.
4. Prioritizes predictable behavior under partial outages through remote caching and policy controls.
5. Runs consistently on **Linux, macOS, and Windows**.

This reframes Chef Server as an integration path, not the core dependency.

---

## 2) Platform targets and compatibility

Primary supported platforms for v1:

- Linux (x86_64/arm64)
- macOS (Apple Silicon + Intel)
- Windows (x86_64)

Cross-platform requirements:

- Use Go standard library path handling (`filepath`) everywhere.
- Avoid shell-specific behavior in core logic (no bash-only assumptions).
- Account for executable naming differences (`.exe` on Windows).
- Use OS-aware default paths while allowing explicit overrides.
- Prefer archive/compression formats with mature cross-platform tooling in Go.

---

## 3) Operating model

At converge time, sushi should evaluate sources according to an **explicit configured order**.

Supported source types:

- `local` — admin-managed cookbook tree on disk.
- `remote` — downloadable cookbook bundle URL with integrated cache fallback.
- `chef_server` — classic server mode (optional, disabled by default).

> Rationale: explicit ordering is clearer and more flexible than presets like `remote-first`.

---

## 4) Non-goals

- Re-implement Chef policy resolution.
- Build a full artifact repository product.
- Replace normal cookbook authoring/testing workflows.

---

## 5) User-facing configuration (v1, JSON)

Define a single JSON config file (for example `/etc/sushi/config.json` on Unix-like systems, or a configurable path on Windows).

```json
{
  "runtime": {
    "client_binary": "auto"
  },
  "source_order": ["local", "remote", "chef_server"],
  "sources": {
    "local": {
      "enabled": true,
      "cookbook_path": "/var/lib/sushi/cookbooks"
    },
    "remote": {
      "enabled": true,
      "url": "https://example.org/cookbooks.tar.zst",
      "checksum_url": "https://example.org/cookbooks.sha256",
      "require_checksum": false,
      "refresh_interval": "6h",
      "cache_dir": "/var/cache/sushi",
      "max_cache_age": "72h",
      "allow_cached_fallback": true,
      "fail_if_stale": true
    },
    "chef_server": {
      "enabled": false,
      "client_rb": "/etc/chef/client.rb",
      "healthcheck": {
        "endpoint": "https://chef.example.com/organizations/acme",
        "timeout": "2s"
      }
    }
  },
  "execution": {
    "run_list_file": "/etc/sushi/run-list.json",
    "json_attributes_file": "/etc/sushi/attributes.json",
    "lock_file": "/var/run/sushi.lock"
  }
}
```

Key decisions:

- Config format is JSON only for v1.
- Source precedence is user-specified via `source_order`.
- Cache is part of the `remote` source behavior, not a separate source.
- Local and remote both produce the same normalized cookbook tree for zero/solo runs.
- Server mode is explicit and optional.

Validation rules:

- `source_order` must contain only known source names (`local`, `remote`, `chef_server`).
- `source_order` entries should be unique.
- At least one enabled source must exist in `source_order`.
- If a source appears in `source_order`, required fields for that source must validate.
- `sources.remote.checksum_url` is required only when `sources.remote.require_checksum=true`.

---

## 6) Execution flow (v1)

1. Load and validate JSON config.
2. Discover usable client binary (`cinc-client` preferred when both exist unless overridden).
3. Iterate through `source_order` and select the first usable source.
4. Materialize cookbook content to a working directory.
5. Generate runtime Chef config snippets as needed.
6. Execute converge and map exit status.
7. Emit structured logs and source-decision metadata.

### Source-resolution detail

For each source in `source_order`:

- If source is disabled, skip with reason.
- If source health/availability checks fail, skip with reason.
- If source is `remote`:
  - attempt fetch/verify from URL;
  - if unavailable and `allow_cached_fallback=true`, use cached bundle when freshness policy allows;
  - if cache is stale and `fail_if_stale=true`, fail this source.
- Select the first source that passes checks.
- If no source qualifies, fail with a clear diagnostic showing all attempted sources and reasons.

---

## 7) Caching + integrity model (remote source)

For remote bundles:

- Download into temp file + atomic rename.
- Verify checksum before activation.
- Expand into versioned cache directories (`<cache_dir>/bundles/<digest>/...`).
- Maintain `current` pointer to active content (symlink/junction/copy fallback by platform).
- Persist metadata:
  - digest,
  - fetched_at,
  - source_url,
  - expires_at (derived from policy).

Use cached remote content only when metadata exists and policy permits.

---

## 8) CLI design (v1)

Proposed subcommands:

- `sushi run` — resolve source + execute converge.
- `sushi fetch` — fetch/verify/activate remote bundle only.
- `sushi doctor` — validate environment, config, connectivity, and cache state.
- `sushi print-plan` — show deterministic decision path without executing converge.

Exit codes should distinguish:

- config invalid,
- dependency missing,
- source unavailable,
- stale cache policy violation,
- converge failure.

---

## 9) Service/daemon operation (cross-platform)

sushi should support non-interactive scheduled/service execution and provide tested examples for each OS:

- **Linux**: `systemd` service + timer examples.
- **macOS**: `launchd` plist examples.
- **Windows**: native Windows Service support implemented directly in the `sushi` binary, including event log integration guidance.

Daemon behavior expectations:

- Graceful start/stop handling.
- Locking to prevent overlapping runs.
- Deterministic retry/backoff behavior.
- Structured logs suitable for each platform’s log collection path.

---

## 10) Observability

Emit both human-readable logs and optional JSON logs with fields like:

- `selected_mode` (`local`, `remote`, `chef_server`),
- `client_binary`,
- `bundle_digest`,
- `cache_age_seconds`,
- `fallback_reason`,
- `chef_exit_code`.

This makes outages and fallback behavior auditable.

---

## 11) Security and safety baseline

- Require HTTPS by default for remote sources (explicit override to allow HTTP).
- Support pinned checksum at minimum.
- Refuse partial/failed extractions.
- File locking to prevent concurrent cache corruption.
- Default to fail-closed on stale cache when `fail_if_stale=true`.

---

## 12) Testing + GitHub workflow plan

Test strategy:

- Unit tests for config parsing/validation, source resolver, binary discovery, and remote cache policy.
- Integration tests for local, remote, and remote-cached fallback behavior.
- Platform-specific tests for path handling, lock behavior, executable resolution, and daemon/service behavior.

CI expectations:

- Run tests in GitHub Actions on **every push**.
- Use a matrix across `ubuntu-latest`, `macos-latest`, and `windows-latest`.
- Require CI success before merging.
- Add `go test ./...` as baseline; expand with race/static checks where feasible.
- Include at least smoke checks for native/OS-appropriate service-mode execution on each platform.

---

## 13) Implementation phases

### Current status snapshot (based on recent commits)

Recently landed work has completed Phase 1 serverless MVP capabilities and supporting artifacts:

- ✅ JSON config schema, parser, and validation are implemented.
- ✅ `doctor`, `run`, and `print-plan` commands are implemented.
- ✅ Explicit `source_order` resolution is implemented with decision reporting.
- ✅ Remote bundle fetch, checksum verification, cache fallback policy, and metadata tracking are implemented.
- ✅ Multi-OS CI workflow exists and runs `go test ./...`.
- ✅ Example JSON configuration has been added under `example/config.json`.

Remaining gaps before Phase 1 can be considered fully complete:

- None currently identified for Phase 1 scope.

Phase 2 hardening is now complete: lock wait/stale handling, converge and request timeout controls, remote retry/backoff, stale-cache warning windows, and expanded test coverage.

Phase 3 optional Chef Server integration is now complete: deterministic `chef_server` resolution (disabled by default), optional healthcheck gating with clear failure reasons, explicit Chef Server run-mode execution, and fallback coverage across `chef_server`, `remote`, and `local` ordering.

Phase 4 operations polish is now complete: cross-platform installation/runbook docs, scheduler/service artifacts (systemd timer, launchd StartInterval, Windows service), bare-command `run` aliasing, platform default config/log paths, retryable converge fallback across sources, and richer operational diagnostics.

### Phase 0 — project scaffolding (short)

- [x] Define JSON config schema + parser + validation.
- [x] Add `doctor` command to report dependency and config readiness.
- [x] Add structured logging skeleton.
- [x] Add baseline multi-OS GitHub Actions workflow (`push`).

### Phase 1 — serverless core (MVP)

- [x] Implement local source + remote bundle fetch + integrated cache fallback.
- [x] Implement explicit `source_order` resolver and decision reporting.
- [x] Implement atomic bundle activation and metadata tracking.
- [x] Implement `run` and `print-plan` commands.
- [x] Execute `chef-client`/`cinc-client` in local/zero mode only.

### Phase 2 — hardening

- [x] Add lock handling, richer retry/backoff, and robust timeout controls.
- [x] Improve integrity checks.
- [x] Add better stale-cache policy controls and warning windows.
- [x] Expand cross-platform test coverage for edge cases.

### Phase 3 — optional Chef Server integration

- [x] Add explicit server mode execution path with healthcheck.
- [x] Keep disabled by default.
- [x] Ensure fallback path remains deterministic and observable under `source_order`.

### Phase 4 — operations polish

- [x] Packaging and installation docs for Linux/macOS/Windows.
- [x] Publish systemd/launchd example configs and implement native Windows Service support in `sushi` (with configuration + Event Log guidance).
- [x] Alias bare `sushi` invocation to `sushi run` to preserve ergonomic default behavior for operators.
- [x] Implement reasonable platform-specific default `config.json` and logging locations for Linux, macOS, and Windows.
- [x] Capture `chef-client`/`cinc-client` output and conditionally attempt the next source in `source_order` when failures match a documented retryable exception list.
- [x] Add service-operation runbooks and platform scheduler/service examples.
- [x] Telemetry hooks and richer diagnostics.

### Phase 5 — plan/code reconciliation gaps

Phase 5 is now complete:

- [x] Add the `sushi fetch` CLI subcommand described in the plan (`run`, `doctor`, `print-plan`, and `version` are currently implemented).
- [x] Implement distinct operational exit codes.
- [x] Add timer/service artifacts and smoke coverage promised in the plan (systemd/launchd examples and native Windows Service implementation/tests are now present in the repository).
- [x] Add runtime output-capture + retryable exception handling to conditionally try the next source after converge-time failures.
- [x] Implement support for common HTTP cache headers (`Cache-Control`, `ETag`, and `Last-Modified`) to avoid unnecessary downloads when remote data is unchanged; fall back to current behavior when headers are absent, and refresh local cache expiration when server responses confirm content is still current.

### Phase 6 — release readiness and policy enforcement

- [ ] Expand CI with explicit smoke checks for service/scheduler artifacts on Linux/macOS/Windows and document acceptance gates.
- [ ] Add end-to-end integration coverage for remote conditional requests (`304`), cache-expiry refresh semantics, and stale-policy exit code behavior.
- [ ] Harden operational contract docs with a dedicated command/exit-code reference and troubleshooting matrix.
- [ ] Add lightweight release automation/versioning guidance for producing signed, reproducible multi-platform binaries.

---

## 14) Definition of done for MVP

MVP is complete when:

1. A node can converge with only local cookbooks and no server.
2. A node can converge by fetching a remote cookbook archive and reusing remote cache during outage.
3. Source selection is driven by explicit `source_order` and is test-covered.
4. Stale remote cache behavior is policy-driven and test-covered.
5. The selected source decision is visible in logs and `print-plan` output.
6. `chef-client` and `cinc-client` selection behavior is deterministic.
7. CI runs tests on Linux, macOS, and Windows for every push.
8. Service/timer examples are provided and smoke-tested across platforms.

---

## 15) Immediate next tasks

1. Expand CI with explicit service/scheduler smoke checks for Linux/macOS/Windows and enforce them as merge gates.
2. Add deeper integration tests for HTTP conditional fetch behavior (`ETag`/`Last-Modified` + `304`) and stale-cache policy exit-code mapping.
3. Prepare release readiness artifacts (versioning/signing/reproducible build notes) for Phase 6 delivery.
