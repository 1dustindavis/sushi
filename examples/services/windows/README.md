# Windows Service Notes

Service management commands:

- Install: `sushi.exe service install -config "%ProgramData%\sushi\config.json"`
- Start: `sushi.exe service start`
- Stop: `sushi.exe service stop`
- Status: `sushi.exe service status`
- Uninstall: `sushi.exe service uninstall`

Event Log guidance:
- Configure service recovery options (`sc.exe failure sushi reset= 86400 actions= restart/5000`).
- Forward `%ProgramData%\sushi\logs\sushi.log` to your preferred log collector and optionally bridge to Windows Event Log via your existing agent pipeline.
