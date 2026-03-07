# sushi

`sushi` is a lightweight wrapper around `chef-client`/`cinc-client` focused on **reliable local convergence without requiring a Chef Server**.

For project direction and implementation planning, see [`docs/PLAN.md`](docs/PLAN.md).

A sample configuration is available at [`example/config.json`](example/config.json).

## Commands

- `sushi run -config <path>`: resolves source order and executes converge flow.
- `sushi doctor -config <path>`: validates config, binary discovery, and source resolution.
- `sushi print-plan -config <path>`: prints source selection decisions without converging.

## Currently implemented

- ✅ **Phase 1 (serverless core/MVP) implemented**: local + remote source resolution, remote bundle fetch with checksum validation, integrated cache fallback policy, atomic cache activation with metadata tracking, `run` execution in local/zero mode, and decision-rich `print-plan`.
