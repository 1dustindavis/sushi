# Command Reference

| Command | Purpose | Notes |
| --- | --- | --- |
| `sushi run -config <path>` | Resolve `source_order` and execute a converge. | Primary non-interactive execution path. |
| `sushi -config <path>` | Alias for `sushi run`. | Useful for operator ergonomics and service wrappers. |
| `sushi fetch -config <path>` | Prefetch/verify/activate remote bundle only. | Does not run Chef converge. |
| `sushi doctor -config <path>` | Validate config, dependency discovery, and source resolution. | Safe preflight command for automation gates. |
| `sushi print-plan -config <path>` | Print deterministic source-selection decisions. | No converge side effects. |
| `sushi archive [<cookbook_dir>] [-o\|--output <archive_path>] [--checksum]` | Package cookbooks as a remote-compatible tarball. | Defaults to current directory; `--checksum` writes `<archive>.sha256`. Does not read config or modify cache. |
| `sushi version` | Print build version string. | Defaults to `dev` unless set by linker flag. |
| `sushi service run -config <path>` | Windows Service host mode. | Windows only. |
| `sushi service <install|uninstall|start|stop|status> [-config <path>]` | Manage native Windows Service lifecycle. | Windows only. |

Linux/macOS should use the platform scheduler (`systemd` timer or `launchd StartInterval`) to run `sushi run` every 10 minutes.

When `-config` is not passed, sushi resolves config paths using platform precedence (macOS configuration profile/defaults, Windows registry, then default file path).
