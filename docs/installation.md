# Installation and Packaging

## Linux

1. Build: `go build -o sushi ./cmd/sushi`
2. Install binary: place `sushi` in `/usr/local/bin` (or managed package path).
3. Install config: `/etc/sushi/config.json`.
4. Logs default to `/var/log/sushi/sushi.log`.
5. Upgrade: replace the binary and keep config/cache directories.
6. Uninstall: remove the binary, config directory, and optional cache/log directories.

## macOS

1. Build: `go build -o sushi ./cmd/sushi`
2. Install binary in `/usr/local/bin`.
3. Preferred managed config: configuration profile at domain `com.github.1dustindavis.sushi` with key `Config` containing the full sushi JSON config string (stored under `/Library/Managed Preferences/`).
4. Optional local defaults override: `defaults write com.github.1dustindavis.sushi Config -string "<json>"`.
5. Fallback config path: `/Library/Application Support/sushi/config.json`.
6. Logs default to `/Library/Logs/sushi/sushi.log`.
7. Upgrade: replace the binary and keep config/cache directories.
8. Uninstall: remove the binary, config directory, and optional cache/log directories.

## Windows

1. Build: `go build -o sushi.exe ./cmd/sushi`
2. Install binary in `C:\Program Files\sushi\sushi.exe`.
3. Preferred managed config: set `HKLM\SOFTWARE\com.github.1dustindavis.sushi\Config` (REG_SZ JSON string).
4. Fallback config path: `%ProgramData%\sushi\config.json`.
5. Logs default to `%ProgramData%\sushi\logs\sushi.log`.
6. Install service: `sushi.exe service install` (or pass `-config` to pin an explicit path).
7. Start service: `sushi.exe service start`.

### Windows service operations

- Check status: `sushi.exe service status`
- Stop: `sushi.exe service stop`
- Start: `sushi.exe service start`
- Uninstall: `sushi.exe service uninstall` (stop first if running)

### Windows upgrade

1. `sushi.exe service stop`
2. Replace `sushi.exe` with the new version.
3. `sushi.exe service start`

### Windows uninstall

1. `sushi.exe service stop`
2. `sushi.exe service uninstall`
3. Remove `C:\Program Files\sushi\sushi.exe` and optional `%ProgramData%\sushi` directories.

You can override defaults with:

- `SUSHI_CONFIG_PATH`
- `SUSHI_LOG_PATH`
