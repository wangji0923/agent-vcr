New-Item -ItemType Directory -Force -Path "src" | Out-Null
Set-Content -Path "src/app.txt" -Value "run-a"
Write-Output "generic fixture run a"
