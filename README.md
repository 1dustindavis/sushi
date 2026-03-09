# sushi

`sushi` is a wrapper around `chef-client`/`cinc-client` focused on **reliable local convergence without requiring a Chef Server**.

A sample configuration is available at [`examples/config.json`](examples/config.json).

## Commands

- `sushi run -config <path>`: resolves source order and executes converge flow.
- `sushi -config <path>`: alias for `sushi run` (operator-friendly default).
- `sushi doctor -config <path>`: validates config, binary discovery, and source resolution.
- `sushi fetch -config <path>`: fetches/verifies/activates the remote bundle without running converge.
- `sushi print-plan -config <path>`: prints source selection decisions without converging.
- `sushi archive [<cookbook_dir>] [-o|--output <archive_path>] [--checksum]`: creates a remote-compatible cookbook archive (defaults to current directory).
- `sushi version`: prints the build version (defaults to `dev` unless set at build time).
- `sushi service <install|uninstall|start|stop|status|run> [-config <path>]`: native Windows service management/host mode.
- `sushi help`: prints command usage.

## Docs

- Configuration reference: [`docs/config-reference.md`](docs/config-reference.md)
- Installation/packaging: [`docs/installation.md`](docs/installation.md)
- Command reference: [`docs/command-reference.md`](docs/command-reference.md)
- Exit-code reference: [`docs/exit-code-reference.md`](docs/exit-code-reference.md)
- Runbook + troubleshooting: [`docs/operations/runbook.md`](docs/operations/runbook.md)
- Service examples: [`examples/services/`](examples/services)
