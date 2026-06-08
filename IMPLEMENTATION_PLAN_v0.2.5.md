# agent-vcr v0.2.5 Implementation Plan

## Scope

v0.2.5 is limited to Behavior Visualization. It consumes v0.2 behavior data and
turns it into static JSON / HTML views for single-run, two-run, and small
multi-run inspection.

Do not implement in v0.2.5:

- HarnessMetadata
- HarnessDiff
- Matrix Compare
- Regression / baseline
- new agent adapters
- SDKs
- LLM Explain
- deterministic replay
- cloud dashboard

## Development Order

### Round 0

Module:

- `docs/v0.2.5/modules/01-visual-domain-model.md`

Status:

- implemented

Purpose:

- lock the public visualization JSON model in `internal/visualize`
- add validation helpers
- add JSON round-trip and architecture tests

### Round 1

These modules can run in parallel after Round 0:

- `02-visual-data-loader`
- `03-swimlane-alignment`
- `04-file-access-compare`
- `05-metrics-cards`

Parallel file ownership:

| Module | Allowed files | Must not modify |
| --- | --- | --- |
| 02 visual data loader | `internal/visualize/load.go`, `internal/visualize/convert.go`, `internal/visualize/load_test.go`, `testdata/visualize/load/**` | `internal/adapters/**`, `internal/trace/**` core schema, `internal/behavior/**` public API, `internal/cli/**` |
| 03 swimlane alignment | `internal/visualize/align.go`, `internal/visualize/path_graph.go`, `internal/visualize/align_test.go`, `testdata/visualize/align/**` | `internal/adapters/**`, `internal/behavior/**` public API, `internal/trace/**`, `internal/cli/**` |
| 04 file access compare | `internal/visualize/file_access.go`, `internal/visualize/file_access_test.go`, `testdata/visualize/file_access/**` | `internal/adapters/**`, `internal/trace/**`, `internal/cli/**` |
| 05 metrics cards | `internal/visualize/metrics_cards.go`, `internal/visualize/metrics_cards_test.go`, `testdata/visualize/metrics/**` | `internal/adapters/**`, `internal/behavior/**` public API, `internal/trace/**`, `internal/cli/**` |

Integration rule:

- Round 1 modules may depend on the locked types in `internal/visualize/model.go`
  and validators in `internal/visualize/validate.go`.
- Round 1 modules must not rename public fields or JSON tags locked in Round 0.

### Round 2

These modules can run in parallel after Round 1 behavior construction is
available:

- `06-html-visual-report`
- `07-cli-integration`

Parallel file ownership:

| Module | Allowed files | Must not modify |
| --- | --- | --- |
| 06 HTML visual report | `internal/visualize/html.go`, `internal/visualize/templates/behavior_visual_report.html.tmpl`, `internal/visualize/html_test.go`, `testdata/golden/visualize/html/**` | `internal/adapters/**`, existing `internal/report/**` unless extracting a shared helper is explicitly approved |
| 07 CLI integration | `internal/cli/visualize.go`, command registration, `internal/visualize/json.go`, `internal/visualize/cli_test.go`, README command examples | `internal/adapters/**`, trace schema, behavior public model |

Integration rule:

- JSON output must write only JSON to stdout.
- HTML output must use Go `html/template` and be static/offline.
- User-controlled content must be escaped.
- Redacted output must not expose prompts, secrets, or raw tool output.

### Round 3

Module:

- `08-testing-e2e-docs`

Allowed files:

- `scripts/e2e-v0.2.5.ps1`
- small `scripts/e2e.ps1` integration
- README
- `docs/behavior-diff.md`
- `docs/release-checklist.md`
- `CHANGELOG.md`
- `testdata/e2e/v0.2.5/**`
- `.github/workflows/**` only for adding the E2E command

Purpose:

- integration review
- E2E fixtures for single-run, two-run, and three-run visualization
- final docs and release checklist

## Locked Public Models

The v0.2.5 public visualization model is locked in `internal/visualize`:

- `VisualReport`
- `VisualRun`
- `BehaviorLane`
- `VisualStep`
- `VisualStepKind`
- `VisualPhase`
- `AlignmentRow`
- `StepCell`
- `DivergenceMarker`
- `FileAccessCompare`
- `FileAccessRow`
- `FileUse`
- `MetricsCard`
- `MetricsCardGroup`
- `VisualOptions`
- `RenderMode`
- `VisualSummary`
- `PathGraph`
- `PathNode`
- `PathEdge`

Compatibility aliases:

- `VisualDivergence = DivergenceMarker`
- `RunMetricsCards = MetricsCardGroup`

Locked helper surface:

- `ValidateReport(report *VisualReport) error`
- `ValidateRunIDs(report *VisualReport) error`

Public model rules:

- all types must be JSON serializable
- JSON field names must remain stable
- no Codex / Claude / Kimi specific fields
- `internal/visualize` must not import `internal/adapters/**`
- `internal/visualize` may consume generic `internal/behavior` and
  `internal/trace` structures
- multi-run lane comparison is supported for user-selected runs, recommended
  maximum 3-5 runs

## Integration Sequence

1. Round 0 locks models and validation.
2. Round 1 loader converts v0.2 behavior data into `VisualRun`,
   `BehaviorLane`, and `VisualStep`.
3. Round 1 alignment builds `AlignmentRow` and `DivergenceMarker`.
4. Round 1 file access builds `FileAccessCompare`.
5. Round 1 metrics builds `MetricsCardGroup`.
6. Round 2 HTML renders the assembled `VisualReport`.
7. Round 2 CLI wires `agent-vcr visualize`.
8. Round 3 E2E validates JSON / HTML / docs / release readiness.

## Test Commands

Round 0 and every following module must run:

```powershell
gofmt -w .
go test ./internal/visualize/...
go test ./...
go vet ./...
go build ./cmd/agent-vcr
```

Round 2 and later must also run:

```powershell
go run ./cmd/agent-vcr visualize --help
```

Round 3 must also run:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\e2e.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\e2e-v0.2.5.ps1
```

## Forbidden Scope

v0.2.5 must not add or implement:

- HarnessMetadata
- HarnessDiff
- Matrix or benchmark runner
- Regression / baseline mode
- new agent adapter
- SDK
- LLM explanation
- deterministic replay
- cloud dashboard
- remote upload

Any implementation that needs those concepts must be deferred to later versions.
