[CmdletBinding()]
param(
  [string]$Version = "0.2.0-dev",
  [string]$DistDir = "dist",
  [switch]$Clean
)

Set-StrictMode -Version 3.0
$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$DistFull = [System.IO.Path]::GetFullPath((Join-Path $RepoRoot $DistDir))

if ($Clean -and (Test-Path $DistFull)) {
  $repoPrefix = $RepoRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
  if (-not $DistFull.StartsWith($repoPrefix, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "refusing to clean outside repository: $DistFull"
  }
  Remove-Item -LiteralPath $DistFull -Recurse -Force
}

New-Item -ItemType Directory -Force $DistFull | Out-Null

$commit = "unknown"
$gitCommit = (& git -C $RepoRoot rev-parse --short HEAD 2>$null)
if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($gitCommit)) {
  $commit = $gitCommit.Trim()
}
$date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$ldflags = "-s -w -X github.com/agent-vcr/agent-vcr/internal/version.Version=$Version -X github.com/agent-vcr/agent-vcr/internal/version.Commit=$commit -X github.com/agent-vcr/agent-vcr/internal/version.Date=$date"

$targets = @(
  @{ GOOS = "linux"; GOARCH = "amd64"; EXT = "" },
  @{ GOOS = "linux"; GOARCH = "arm64"; EXT = "" },
  @{ GOOS = "darwin"; GOARCH = "amd64"; EXT = "" },
  @{ GOOS = "darwin"; GOARCH = "arm64"; EXT = "" },
  @{ GOOS = "windows"; GOARCH = "amd64"; EXT = ".exe" }
)

$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH
$oldCgo = $env:CGO_ENABLED
$archives = @()

try {
  foreach ($target in $targets) {
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    $env:CGO_ENABLED = "0"

    $name = "agent-vcr_${Version}_$($target.GOOS)_$($target.GOARCH)"
    $targetDir = Join-Path $DistFull $name
    New-Item -ItemType Directory -Force $targetDir | Out-Null

    $binary = Join-Path $targetDir ("agent-vcr" + $target.EXT)
    Write-Host "building $name"
    Push-Location $RepoRoot
    try {
      & go build -trimpath -ldflags $ldflags -o $binary ./cmd/agent-vcr
      if ($LASTEXITCODE -ne 0) {
        throw "go build failed for $name"
      }
    } finally {
      Pop-Location
    }

    $readme = Join-Path $RepoRoot "README.md"
    if (Test-Path $readme) {
      Copy-Item -LiteralPath $readme -Destination (Join-Path $targetDir "README.md")
    }

    if ($target.GOOS -eq "windows") {
      $archive = Join-Path $DistFull ($name + ".zip")
      Compress-Archive -Path (Join-Path $targetDir "*") -DestinationPath $archive -Force
    } else {
      $archive = Join-Path $DistFull ($name + ".tar.gz")
      Push-Location $DistFull
      try {
        & tar -czf (Split-Path -Leaf $archive) $name
        if ($LASTEXITCODE -ne 0) {
          throw "tar failed for $name"
        }
      } finally {
        Pop-Location
      }
    }
    $archives += $archive
  }
} finally {
  $env:GOOS = $oldGoos
  $env:GOARCH = $oldGoarch
  $env:CGO_ENABLED = $oldCgo
}

$checksumPath = Join-Path $DistFull "checksums.txt"
$lines = foreach ($archive in $archives) {
  $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $archive).Hash.ToLowerInvariant()
  "$hash  $(Split-Path -Leaf $archive)"
}
Set-Content -Path $checksumPath -Value $lines
Write-Host "wrote $checksumPath"
