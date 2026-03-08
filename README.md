# sushi

`sushi` is a wrapper around `chef-client`/`cinc-client` focused on **reliable local convergence without requiring a Chef Server**.

For project direction and implementation planning, see [`docs/PLAN.md`](docs/PLAN.md).

A sample configuration is available at [`example/config.json`](example/config.json).

## Commands

- `sushi run -config <path>`: resolves source order and executes converge flow.
- `sushi doctor -config <path>`: validates config, binary discovery, and source resolution.
- `sushi print-plan -config <path>`: prints source selection decisions without converging.
- `sushi version`: prints the build version (defaults to `dev` unless set at build time).
- `sushi help`: prints command usage.

## Currently implemented

- ✅ **Phase 1 (serverless core/MVP) implemented**: local + remote source resolution, remote bundle fetch, integrated cache fallback policy, atomic cache activation with metadata tracking, `run` execution in local/zero mode, and decision-rich `print-plan`.
- ✅ **Phase 2 (hardening) implemented**: lock/timeout controls, retry/backoff, stronger remote integrity + stale-cache policy controls, and expanded unit test coverage.
- ✅ **Phase 3 (optional Chef Server integration) implemented**: deterministic `chef_server` source resolution with optional healthchecks, explicit Chef Server execution mode, and fallback visibility in `print-plan`/`doctor`.
