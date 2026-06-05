[CmdletBinding()]
param(
  [string]$AgentVCR,
  [switch]$KeepTemp
)

Set-StrictMode -Version 3.0
$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$IsWin = [System.Environment]::OSVersion.Platform -eq [System.PlatformID]::Win32NT
$TempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("agent-vcr-e2e-" + [guid]::NewGuid().ToString("N"))
$FixtureRepo = Join-Path $TempRoot "repo"
$Utf8NoBom = [System.Text.UTF8Encoding]::new($false)

function Write-Step {
  param([string]$Message)
  Write-Host "[e2e] $Message"
}

function Fail {
  param([string]$Message)
  throw "[e2e] $Message"
}

function Invoke-AgentVCR {
  param(
    [Parameter(Mandatory = $true)]
    [string[]]$Arguments,
    [string]$InputText
  )

  if ($PSBoundParameters.ContainsKey("InputText")) {
    $output = $InputText | & $AgentVCR @Arguments
  } else {
    $output = & $AgentVCR @Arguments
  }
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

function Assert-NotContains {
  param(
    [string]$Text,
    [string]$Pattern,
    [string]$Label
  )
  if ($Text -match $Pattern) {
    Fail "$Label unexpectedly matched /$Pattern/`n$Text"
  }
}

function Parse-RunID {
  param([string]$Text)
  if ($Text -notmatch "recorded run\s+(\S+)\s+\(([^)]+)\)") {
    Fail "could not parse run id from output:`n$Text"
  }
  $Matches[1]
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

function New-TraceEvent {
  param(
    [Parameter(Mandatory = $true)]
    [string]$RunID,
    [Parameter(Mandatory = $true)]
    [int]$Index,
    [Parameter(Mandatory = $true)]
    [string]$Type,
    [Parameter(Mandatory = $true)]
    [datetime]$StartedAt,
    [hashtable]$Payload = @{}
  )

  [ordered]@{
    schema_version = "0.2"
    event_id       = "evt-$RunID-$Index"
    run_id         = $RunID
    event_index    = $Index
    type           = $Type
    source         = [ordered]@{
      adapter        = "e2e-fixture"
      raw_event_type = $Type
    }
    timestamp      = $StartedAt.AddSeconds($Index).ToUniversalTime().ToString("o")
    payload        = $Payload
  }
}

function New-BehaviorFixtureRun {
  param(
    [Parameter(Mandatory = $true)]
    [string]$Repo,
    [Parameter(Mandatory = $true)]
    [string]$RunID,
    [Parameter(Mandatory = $true)]
    [datetime]$StartedAt,
    [Parameter(Mandatory = $true)]
    [array]$Events
  )

  $runDir = Join-Path $Repo ".agent-vcr/runs/$RunID"
  New-Item -ItemType Directory -Force `
    -Path $runDir, (Join-Path $runDir "blobs"), (Join-Path $runDir "patches"), (Join-Path $runDir "raw") |
    Out-Null

  $endedAt = $StartedAt.AddSeconds($Events.Count + 1).ToUniversalTime().ToString("o")
  $metadata = [ordered]@{
    schema_version = "0.2"
    run_id         = $RunID
    source         = "behavior-e2e"
    status         = "completed"
    cwd            = $Repo
    repo_root      = $Repo
    started_at     = $StartedAt.ToUniversalTime().ToString("o")
    ended_at       = $endedAt
    summary        = [ordered]@{
      fixture = "v0.2 behavior"
    }
  }
  Set-TextNoBom -Path (Join-Path $runDir "metadata.json") -Text ($metadata | ConvertTo-Json -Depth 20)

  $tracePath = Join-Path $runDir "trace.ndjson"
  if (Test-Path $tracePath) {
    Remove-Item -LiteralPath $tracePath -Force
  }
  foreach ($event in $Events) {
    Add-TextLineNoBom -Path $tracePath -Text ($event | ConvertTo-Json -Depth 20 -Compress)
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

function Assert-JsonParses {
  param(
    [string]$Text,
    [string]$Label
  )
  $null = Convert-JsonOrFail -Text $Text -Label $Label
}

try {
  New-Item -ItemType Directory -Force $TempRoot, $FixtureRepo | Out-Null

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

  $shellExe = (Get-Process -Id $PID).Path
  if ([string]::IsNullOrWhiteSpace($shellExe)) {
    $shellExe = if ($IsWin) { "powershell.exe" } else { "pwsh" }
  }
  $shellArgs = @("-NoProfile")
  if ($IsWin) {
    $shellArgs += @("-ExecutionPolicy", "Bypass")
  }

  Write-Step "creating temporary git repo"
  & git -C $FixtureRepo init -q
  if ($LASTEXITCODE -ne 0) { Fail "git init failed" }
  & git -C $FixtureRepo config user.email "agent-vcr-e2e@example.invalid"
  & git -C $FixtureRepo config user.name "agent-vcr e2e"
  Set-Content -Path (Join-Path $FixtureRepo "README.md") -Value "agent-vcr e2e fixture"
  & git -C $FixtureRepo add README.md
  & git -C $FixtureRepo commit -m "initial fixture" -q
  if ($LASTEXITCODE -ne 0) { Fail "git commit failed" }

  Write-Step "initializing Codex hook config"
  Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "init", "codex", "--force") | Out-Null
  & git -C $FixtureRepo check-ignore .agent-vcr/ | Out-Null
  if ($LASTEXITCODE -ne 0) {
    Fail ".agent-vcr/ is not gitignored"
  }

  Write-Step "simulating Codex hook events"
  $cwdJson = $FixtureRepo | ConvertTo-Json -Compress
  foreach ($fixture in @(
    "codex-session-start.json",
    "codex-pre-tool-bash.json",
    "codex-post-tool-bash.json",
    "codex-stop.json"
  )) {
    $template = Get-Content -Raw (Join-Path $RepoRoot "testdata/e2e/$fixture")
    $inputJson = $template.Replace("__CWD_JSON__", $cwdJson)
    $result = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "hook", "--adapter", "codex") -InputText $inputJson
    if (-not [string]::IsNullOrWhiteSpace($result.Text)) {
      Fail "hook wrote stdout for $fixture`n$result.Text"
    }
  }

  Write-Step "recording Generic CLI fixture A"
  Push-Location $FixtureRepo
  try {
    $fixtureA = Join-Path $RepoRoot "testdata/e2e/generic-a.ps1"
    $fixtureB = Join-Path $RepoRoot "testdata/e2e/generic-b.ps1"

    $runAResult = Invoke-AgentVCR -Arguments (@("--project-dir", $FixtureRepo, "record", "--adapter", "generic-cli", "--name", "generic-a", "--") + @($shellExe) + $shellArgs + @("-File", $fixtureA))
    $runA = Parse-RunID $runAResult.Text

    Write-Step "recording Generic CLI fixture B"
    $runBResult = Invoke-AgentVCR -Arguments (@("--project-dir", $FixtureRepo, "record", "--adapter", "generic-cli", "--name", "generic-b", "--") + @($shellExe) + $shellArgs + @("-File", $fixtureB))
    $runB = Parse-RunID $runBResult.Text
  } finally {
    Pop-Location
  }

  Write-Step "running list"
  $list = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "list")
  Assert-Contains $list.Text "generic-cli" "list output"
  Assert-Contains $list.Text "codex-hooks" "list output"

  Write-Step "running replay"
  $replay = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "replay", "latest")
  Assert-Contains $replay.Text "Run:" "replay output"

  Write-Step "running diff"
  $diff = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "diff", $runA, $runB)
  Assert-Contains $diff.Text "Run A:" "diff output"

  Write-Step "running check"
  $check = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "check", "latest")
  Assert-Contains $check.Text "Status:" "check output"

  Write-Step "probing export --html"
  $exportHelp = Invoke-AgentVCR -Arguments @("export", "--help")
  if ($exportHelp.Text -match "--html") {
    $export = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "export", "latest", "--html")
    if ([string]::IsNullOrWhiteSpace($export.Text)) {
      Write-Step "export --html completed"
    } else {
      Write-Step "export --html completed: $($export.Text)"
    }
  } else {
    Write-Step "SKIP export --html: command is not available in this checkout"
  }

  Write-Step "constructing v0.2 behavior fixture runs"
  $behaviorA = "behavior-a"
  $behaviorB = "behavior-b"
  $behaviorBase = (Get-Date).ToUniversalTime().AddMinutes(5)

  New-BehaviorFixtureRun -Repo $FixtureRepo -RunID $behaviorA -StartedAt $behaviorBase -Events @(
    (New-TraceEvent -RunID $behaviorA -Index 1 -Type "shell_command" -StartedAt $behaviorBase -Payload @{
      command = 'rg "session" src tests'
      tool_use_id = "a-search"
    }),
    (New-TraceEvent -RunID $behaviorA -Index 2 -Type "shell_result" -StartedAt $behaviorBase -Payload @{
      command = 'rg "session" src tests'
      tool_use_id = "a-search"
      exit_code = 0
    }),
    (New-TraceEvent -RunID $behaviorA -Index 3 -Type "file_read" -StartedAt $behaviorBase -Payload @{
      target = "tests/auth/session.test.ts"
      files = @("tests/auth/session.test.ts")
    }),
    (New-TraceEvent -RunID $behaviorA -Index 4 -Type "file_patch" -StartedAt $behaviorBase -Payload @{
      target = "src/auth/session.ts"
      files = @("src/auth/session.ts")
    }),
    (New-TraceEvent -RunID $behaviorA -Index 5 -Type "shell_command" -StartedAt $behaviorBase -Payload @{
      command = "npm test"
      tool_use_id = "a-test"
    }),
    (New-TraceEvent -RunID $behaviorA -Index 6 -Type "shell_result" -StartedAt $behaviorBase -Payload @{
      command = "npm test"
      tool_use_id = "a-test"
      exit_code = 0
    }),
    (New-TraceEvent -RunID $behaviorA -Index 7 -Type "run_stop" -StartedAt $behaviorBase -Payload @{
      status = "success"
    })
  )

  $behaviorBStart = $behaviorBase.AddMinutes(1)
  New-BehaviorFixtureRun -Repo $FixtureRepo -RunID $behaviorB -StartedAt $behaviorBStart -Events @(
    (New-TraceEvent -RunID $behaviorB -Index 1 -Type "shell_command" -StartedAt $behaviorBStart -Payload @{
      command = 'rg "cookie" src'
      tool_use_id = "b-search"
    }),
    (New-TraceEvent -RunID $behaviorB -Index 2 -Type "shell_result" -StartedAt $behaviorBStart -Payload @{
      command = 'rg "cookie" src'
      tool_use_id = "b-search"
      exit_code = 0
    }),
    (New-TraceEvent -RunID $behaviorB -Index 3 -Type "file_read" -StartedAt $behaviorBStart -Payload @{
      target = "src/auth/legacy-cookie.ts"
      files = @("src/auth/legacy-cookie.ts")
    }),
    (New-TraceEvent -RunID $behaviorB -Index 4 -Type "file_patch" -StartedAt $behaviorBStart -Payload @{
      target = "src/auth/legacy-cookie.ts"
      files = @("src/auth/legacy-cookie.ts")
    }),
    (New-TraceEvent -RunID $behaviorB -Index 5 -Type "run_stop" -StartedAt $behaviorBStart -Payload @{
      status = "completed"
    })
  )

  Write-Step "running behavior latest"
  $behaviorLatest = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "behavior", "latest")
  Assert-Contains $behaviorLatest.Text "Behavior|behavior" "behavior latest output"

  Write-Step "running behavior latest --json"
  $behaviorLatestJSON = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "behavior", "latest", "--json")
  Assert-JsonParses $behaviorLatestJSON.Text "behavior latest --json output"

  Write-Step "running behavior diff"
  $behaviorDiff = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "behavior", "diff", $behaviorA, $behaviorB)
  Assert-Contains $behaviorDiff.Text "first behavior divergence|First behavior divergence|first_behavior_divergence" "behavior diff output"
  Assert-NotContains $behaviorDiff.Text "unavailable|not integrated|waiting for module" "behavior diff output"

  Write-Step "running behavior diff --json"
  $behaviorDiffJSON = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "behavior", "diff", $behaviorA, $behaviorB, "--json")
  $behaviorDiffObject = Convert-JsonOrFail -Text $behaviorDiffJSON.Text -Label "behavior diff --json output"
  Assert-Contains $behaviorDiffJSON.Text "first_divergence|divergence" "behavior diff --json output"
  if ($null -eq $behaviorDiffObject.first_divergence) {
    Fail "behavior diff --json did not include first_divergence`n$($behaviorDiffJSON.Text)"
  }
  if ([string]$behaviorDiffObject.first_divergence.kind -eq "no_divergence") {
    Fail "behavior diff --json reported no_divergence for divergent fixture`n$($behaviorDiffJSON.Text)"
  }

  Write-Step "running behavior metrics"
  $behaviorMetrics = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "behavior", "metrics", "latest")
  Assert-Contains $behaviorMetrics.Text "Context|Validation|Metrics|metrics" "behavior metrics output"

  Write-Step "running behavior metrics --json"
  $behaviorMetricsJSON = Invoke-AgentVCR -Arguments @("--project-dir", $FixtureRepo, "behavior", "metrics", "latest", "--json")
  Assert-JsonParses $behaviorMetricsJSON.Text "behavior metrics --json output"

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
