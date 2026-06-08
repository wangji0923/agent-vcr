[CmdletBinding()]
param(
  [string]$AgentVCR,
  [switch]$KeepTemp
)

Set-StrictMode -Version 3.0
$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$IsWin = [System.Environment]::OSVersion.Platform -eq [System.PlatformID]::Win32NT
$TempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("agent-vcr-visualize-e2e-" + [guid]::NewGuid().ToString("N"))
$FixtureRepo = Join-Path $TempRoot "repo"
$OutputDir = Join-Path $TempRoot "out"
$FixturePath = Join-Path $RepoRoot "testdata/e2e/v0.2.5/visual-runs.json"
$Utf8NoBom = [System.Text.UTF8Encoding]::new($false)

if ($IsWin -and (Test-Path "C:\Program Files\Go\bin")) {
  $env:Path = "C:\Program Files\Go\bin;$env:Path"
}

function Write-Step {
  param([string]$Message)
  Write-Host "[e2e-v0.2.5] $Message"
}

function Fail {
  param([string]$Message)
  throw "[e2e-v0.2.5] $Message"
}

function Set-TextNoBom {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Path,
    [Parameter(Mandatory = $true)]
    [string]$Text
  )
  [System.IO.File]::WriteAllText($Path, $Text, $Utf8NoBom)
}

function Add-TextLineNoBom {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Path,
    [Parameter(Mandatory = $true)]
    [string]$Text
  )
  [System.IO.File]::AppendAllText($Path, $Text + [Environment]::NewLine, $Utf8NoBom)
}

function Invoke-AgentVCR {
  param(
    [Parameter(Mandatory = $true)]
    [string[]]$Arguments
  )

  $output = & $AgentVCR @Arguments
  $code = $LASTEXITCODE
  $text = ($output | ForEach-Object { [string]$_ }) -join "`n"
  if ($code -ne 0) {
    Fail "agent-vcr $($Arguments -join ' ') exited $code`n$text"
  }
  [pscustomobject]@{
    Code = $code
    Text = $text
  }
}

function Convert-JsonOrFail {
  param(
    [string]$Text,
    [string]$Label
  )
  try {
    return $Text | ConvertFrom-Json -ErrorAction Stop
  } catch {
    Fail "$Label is not parseable JSON: $($_.Exception.Message)`n$Text"
  }
}

function Assert-Contains {
  param(
    [string]$Text,
    [string]$Pattern,
    [string]$Label
  )
  if ($Text -notmatch $Pattern) {
    Fail "$Label did not match /$Pattern/`n$Text"
  }
}

function Assert-CountAtLeast {
  param(
    [object]$Value,
    [int]$Minimum,
    [string]$Label
  )
  $count = @($Value).Count
  if ($count -lt $Minimum) {
    Fail "$Label count $count is less than $Minimum"
  }
}

function Assert-FileNonEmpty {
  param(
    [string]$Path,
    [string]$Label
  )
  if (-not (Test-Path $Path)) {
    Fail "$Label was not created: $Path"
  }
  $item = Get-Item $Path
  if ($item.Length -le 0) {
    Fail "$Label is empty: $Path"
  }
}

function Assert-VisualReport {
  param(
    [object]$Report,
    [int]$RunCount,
    [string]$Mode,
    [string]$Label
  )
  if ([string]$Report.schema_version -ne "0.2.5") {
    Fail "$Label schema_version is not 0.2.5"
  }
  if ([string]$Report.mode -ne $Mode) {
    Fail "$Label mode is $($Report.mode), want $Mode"
  }
  if ([int]$Report.summary.run_count -ne $RunCount) {
    Fail "$Label run_count is $($Report.summary.run_count), want $RunCount"
  }
  Assert-CountAtLeast -Value $Report.runs -Minimum $RunCount -Label "$Label runs"
  Assert-CountAtLeast -Value $Report.lanes -Minimum $RunCount -Label "$Label lanes"
}

function Assert-HasMetrics {
  param(
    [object]$Report,
    [string]$Label
  )
  Assert-CountAtLeast -Value $Report.metrics -Minimum 1 -Label "$Label metric groups"
  $cardCount = 0
  foreach ($group in @($Report.metrics)) {
    $cardCount += @($group.cards).Count
  }
  if ($cardCount -le 0) {
    Fail "$Label did not include metrics cards"
  }
}

function Assert-HasFileAccess {
  param(
    [object]$Report,
    [string]$Path,
    [string]$Label
  )
  $rows = @($Report.file_access.rows)
  Assert-CountAtLeast -Value $rows -Minimum 1 -Label "$Label file access rows"
  foreach ($row in $rows) {
    if ([string]$row.path -eq $Path) {
      return
    }
  }
  Fail "$Label did not include file access row for $Path"
}

function Assert-HasFirstDivergence {
  param(
    [object]$Report,
    [string]$Label
  )
  if ($null -ne $Report.summary.first_divergence) {
    return
  }
  foreach ($marker in @($Report.divergences)) {
    if ($marker.first -eq $true) {
      return
    }
  }
  Fail "$Label did not include a first divergence marker"
}

function Assert-HtmlContains {
  param(
    [string]$Path,
    [string[]]$Patterns,
    [string]$Label
  )
  Assert-FileNonEmpty -Path $Path -Label $Label
  $html = Get-Content -Raw -LiteralPath $Path
  foreach ($pattern in $Patterns) {
    Assert-Contains -Text $html -Pattern $pattern -Label $Label
  }
}

function Get-PropertyValue {
  param(
    [object]$Object,
    [string]$Name
  )
  if ($null -eq $Object) {
    return $null
  }
  $property = $Object.PSObject.Properties[$Name]
  if ($null -eq $property) {
    return $null
  }
  $property.Value
}

function Add-PayloadValue {
  param(
    [System.Collections.IDictionary]$Payload,
    [object]$FixtureEvent,
    [string]$Name
  )
  $value = Get-PropertyValue -Object $FixtureEvent -Name $Name
  if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) {
    $Payload[$Name] = $value
  }
}

function New-TraceEvent {
  param(
    [Parameter(Mandatory = $true)]
    [string]$RunID,
    [Parameter(Mandatory = $true)]
    [int]$Index,
    [Parameter(Mandatory = $true)]
    [object]$FixtureEvent,
    [Parameter(Mandatory = $true)]
    [datetime]$StartedAt
  )

  $type = [string](Get-PropertyValue -Object $FixtureEvent -Name "type")
  if ([string]::IsNullOrWhiteSpace($type)) {
    Fail "fixture event $Index for $RunID is missing type"
  }

  $payload = [ordered]@{}
  foreach ($name in @("command", "target", "status", "tool_use_id", "tool_name", "query")) {
    Add-PayloadValue -Payload $payload -FixtureEvent $FixtureEvent -Name $name
  }
  $files = Get-PropertyValue -Object $FixtureEvent -Name "files"
  if ($null -ne $files) {
    $payload["files"] = @($files)
  }
  $exitCode = Get-PropertyValue -Object $FixtureEvent -Name "exit_code"
  if ($null -ne $exitCode) {
    $payload["exit_code"] = [int]$exitCode
  }

  [ordered]@{
    schema_version = "0.2"
    event_id       = "evt-$RunID-$Index"
    run_id         = $RunID
    event_index    = $Index
    type           = $type
    source         = [ordered]@{
      adapter        = "visualize-e2e"
      raw_event_type = $type
    }
    timestamp      = $StartedAt.AddSeconds($Index).ToUniversalTime().ToString("o")
    payload        = $payload
  }
}

function New-FixtureRepoFile {
  param([string]$RelativePath)
  if ([string]::IsNullOrWhiteSpace($RelativePath)) {
    return
  }
  $path = Join-Path $FixtureRepo $RelativePath
  $parent = Split-Path -Parent $path
  if (-not [string]::IsNullOrWhiteSpace($parent)) {
    New-Item -ItemType Directory -Force -Path $parent | Out-Null
  }
  if (-not (Test-Path $path)) {
    Set-TextNoBom -Path $path -Text "// e2e fixture: $RelativePath`n"
  }
}

function New-VisualFixtureRun {
  param(
    [Parameter(Mandatory = $true)]
    [object]$Run,
    [Parameter(Mandatory = $true)]
    [datetime]$BaseStartedAt
  )

  $runID = [string]$Run.run_id
  $offset = [int]$Run.start_offset_seconds
  $startedAt = $BaseStartedAt.AddSeconds($offset)
  $events = @($Run.events)
  $runDir = Join-Path $FixtureRepo ".agent-vcr/runs/$runID"
  New-Item -ItemType Directory -Force `
    -Path $runDir, (Join-Path $runDir "blobs"), (Join-Path $runDir "patches"), (Join-Path $runDir "raw") |
    Out-Null

  $endedAt = $startedAt.AddSeconds($events.Count + 1)
  $metadata = [ordered]@{
    schema_version = "0.2"
    run_id         = $runID
    source         = [string]$Run.source
    status         = [string]$Run.status
    cwd            = $FixtureRepo
    repo_root      = $FixtureRepo
    started_at     = $startedAt.ToUniversalTime().ToString("o")
    ended_at       = $endedAt.ToUniversalTime().ToString("o")
    summary        = [ordered]@{
      fixture = "v0.2.5 behavior visualization"
      label   = [string]$Run.label
    }
  }
  Set-TextNoBom -Path (Join-Path $runDir "metadata.json") -Text ($metadata | ConvertTo-Json -Depth 20)

  $tracePath = Join-Path $runDir "trace.ndjson"
  if (Test-Path $tracePath) {
    Remove-Item -LiteralPath $tracePath -Force
  }
  $index = 1
  foreach ($event in $events) {
    foreach ($file in @((Get-PropertyValue -Object $event -Name "files"))) {
      New-FixtureRepoFile -RelativePath ([string]$file)
    }
    $target = Get-PropertyValue -Object $event -Name "target"
    if ($null -ne $target) {
      New-FixtureRepoFile -RelativePath ([string]$target)
    }
    $traceEvent = New-TraceEvent -RunID $runID -Index $index -FixtureEvent $event -StartedAt $startedAt
    Add-TextLineNoBom -Path $tracePath -Text ($traceEvent | ConvertTo-Json -Depth 20 -Compress)
    $index++
  }
}

try {
  New-Item -ItemType Directory -Force $TempRoot, $FixtureRepo, $OutputDir | Out-Null

  if ([string]::IsNullOrWhiteSpace($AgentVCR)) {
    $exeName = if ($IsWin) { "agent-vcr.exe" } else { "agent-vcr" }
    $AgentVCR = Join-Path $TempRoot $exeName
    Write-Step "building agent-vcr"
    Push-Location $RepoRoot
    try {
      & go build -o $AgentVCR ./cmd/agent-vcr
      if ($LASTEXITCODE -ne 0) {
        Fail "go build failed"
      }
    } finally {
      Pop-Location
    }
  } else {
    $AgentVCR = (Resolve-Path $AgentVCR).Path
  }

  Write-Step "checking visualize command"
  Invoke-AgentVCR -Arguments @("visualize", "--help") | Out-Null

  Write-Step "creating temporary fixture repo"
  & git -C $FixtureRepo init -q
  if ($LASTEXITCODE -ne 0) { Fail "git init failed" }
  & git -C $FixtureRepo config user.email "agent-vcr-e2e@example.invalid"
  & git -C $FixtureRepo config user.name "agent-vcr e2e"
  Set-TextNoBom -Path (Join-Path $FixtureRepo "README.md") -Text "agent-vcr v0.2.5 visualize e2e fixture`n"
  & git -C $FixtureRepo add README.md
  & git -C $FixtureRepo commit -m "initial fixture" -q
  if ($LASTEXITCODE -ne 0) { Fail "git commit failed" }

  Write-Step "constructing visualization runs"
  $fixture = Get-Content -Raw -LiteralPath $FixturePath | ConvertFrom-Json -ErrorAction Stop
  $baseStartedAt = (Get-Date).ToUniversalTime().AddMinutes(10)
  foreach ($run in @($fixture.runs)) {
    New-VisualFixtureRun -Run $run -BaseStartedAt $baseStartedAt
  }

  $runA = "visual-a-test-first"
  $runB = "visual-b-legacy-no-test"
  $runC = "visual-c-source-first"

  Write-Step "running single-run visualize --json"
  $singleJSON = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "visualize", $runA, "--json", "--no-cache")
  $singleReport = Convert-JsonOrFail -Text $singleJSON.Text -Label "single-run visualize --json output"
  Assert-VisualReport -Report $singleReport -RunCount 1 -Mode "single" -Label "single-run JSON"
  Assert-HasFileAccess -Report $singleReport -Path "src/auth/session.ts" -Label "single-run JSON"
  Assert-HasMetrics -Report $singleReport -Label "single-run JSON"

  Write-Step "running latest visualize --json"
  $latestJSON = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "visualize", "latest", "--json", "--no-cache")
  $latestReport = Convert-JsonOrFail -Text $latestJSON.Text -Label "latest visualize --json output"
  Assert-VisualReport -Report $latestReport -RunCount 1 -Mode "single" -Label "latest JSON"
  $latestRun = @($latestReport.runs)[0]
  if ([string]$latestRun.run_id -ne $runC) {
    Fail "latest visualize resolved to $($latestRun.run_id), want $runC"
  }

  Write-Step "running single-run visualize --html"
  $singleHTML = Join-Path $OutputDir "single.html"
  Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "visualize", $runA, "--html", "--output", $singleHTML, "--no-cache") | Out-Null
  Assert-HtmlContains -Path $singleHTML -Label "single-run HTML" -Patterns @(
    "Swimlane|Lane|Timeline|Behavior",
    "File access|File Access|file access",
    "Metrics|metrics"
  )

  Write-Step "running two-run visualize --json"
  $compareJSON = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "visualize", $runA, $runB, "--json", "--no-cache")
  $compareReport = Convert-JsonOrFail -Text $compareJSON.Text -Label "two-run visualize --json output"
  Assert-VisualReport -Report $compareReport -RunCount 2 -Mode "compare" -Label "two-run JSON"
  Assert-CountAtLeast -Value $compareReport.alignment -Minimum 1 -Label "two-run alignment"
  Assert-HasFirstDivergence -Report $compareReport -Label "two-run JSON"
  Assert-HasFileAccess -Report $compareReport -Path "src/auth/session.ts" -Label "two-run JSON"
  Assert-HasFileAccess -Report $compareReport -Path "src/auth/legacy-cookie.ts" -Label "two-run JSON"
  Assert-HasMetrics -Report $compareReport -Label "two-run JSON"

  Write-Step "running two-run visualize --html"
  $compareHTML = Join-Path $OutputDir "compare.html"
  Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "visualize", $runA, $runB, "--html", "--output", $compareHTML, "--no-cache") | Out-Null
  Assert-HtmlContains -Path $compareHTML -Label "two-run HTML" -Patterns @(
    "First divergence|first divergence|first_divergence",
    "Swimlane|Lane|lanes|Behavior",
    "File access|File Access|file access",
    "Metrics|metrics",
    "legacy-cookie",
    "session.ts"
  )

  Write-Step "running multi-run visualize --json"
  $multiJSON = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "visualize", $runA, $runB, $runC, "--json", "--max-runs", "5", "--no-cache")
  $multiReport = Convert-JsonOrFail -Text $multiJSON.Text -Label "multi-run visualize --json output"
  Assert-VisualReport -Report $multiReport -RunCount 3 -Mode "compare" -Label "multi-run JSON"
  Assert-CountAtLeast -Value $multiReport.alignment -Minimum 1 -Label "multi-run alignment"
  Assert-HasFirstDivergence -Report $multiReport -Label "multi-run JSON"
  Assert-HasFileAccess -Report $multiReport -Path "src/auth/session.ts" -Label "multi-run JSON"
  Assert-HasFileAccess -Report $multiReport -Path "src/auth/legacy-cookie.ts" -Label "multi-run JSON"
  Assert-HasMetrics -Report $multiReport -Label "multi-run JSON"

  Write-Step "running multi-run visualize --html"
  $multiHTML = Join-Path $OutputDir "multi.html"
  Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "visualize", $runA, $runB, $runC, "--html", "--output", $multiHTML, "--max-runs", "5", "--no-cache") | Out-Null
  Assert-HtmlContains -Path $multiHTML -Label "multi-run HTML" -Patterns @(
    "Swimlane|Lane|lanes|Behavior",
    "File access|File Access|file access",
    "Metrics|metrics",
    "test-first",
    "legacy-no-test",
    "visual-c-source-first"
  )

  Write-Step "PASS temp=$TempRoot"
} finally {
  if (-not $KeepTemp -and (Test-Path $TempRoot)) {
    $fullTemp = [System.IO.Path]::GetFullPath($TempRoot)
    $systemTemp = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
    if ($fullTemp.StartsWith($systemTemp, [System.StringComparison]::OrdinalIgnoreCase)) {
      Remove-Item -LiteralPath $fullTemp -Recurse -Force
    }
  }
}
