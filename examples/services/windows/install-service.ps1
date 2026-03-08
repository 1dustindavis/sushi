$binary = "C:\Program Files\sushi\sushi.exe"
& $binary service install -config "$env:ProgramData\sushi\config.json"
& $binary service start
