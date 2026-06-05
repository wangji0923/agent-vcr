New-Item -ItemType Directory -Force -Path "src", "tests" | Out-Null
Set-Content -Path "src/app.txt" -Value "run-b"
Set-Content -Path "tests/app.test.txt" -Value "run-b test"
Write-Output "generic fixture run b"
