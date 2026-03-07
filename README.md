# sushi

`sushi` is a lightweight wrapper around `chef-client`/`cinc-client` focused on **reliable local convergence without requiring a Chef Server**.

For project direction and implementation planning, see [`docs/PLAN.md`](docs/PLAN.md).

## Commands

- `sushi run -config <path>`: resolves source order and executes converge flow.
- `sushi doctor -config <path>`: validates config, binary discovery, and source resolution.
- `sushi print-plan -config <path>`: prints source selection decisions without converging.

## Currently implemented

- JSON configuration file support with validation for source ordering and required per-source settings.
- Automatic Chef client detection (`cinc-client` preferred, with `chef-client` fallback) and explicit binary override via config.
- Deterministic source selection from `source_order` across `local`, `remote`, and `chef_server` entries.
- A `doctor` command that checks whether the configured environment is ready to run.
- A `print-plan` command that shows source selection decisions before execution.
