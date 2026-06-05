# agent-vcr v0.2 Implementation Plan

## Scope

v0.2 is limited to BehaviorSignature, behavior.Step extraction, first behavior
divergence, and behavior metrics.

Do not implement in v0.2:

- HarnessMetadata
- HarnessDiff
- Matrix or benchmark runner
- Regression platform
- LLM Explain
- deterministic replay
- new agent adapters

## Locked Public Interfaces

The v0.2 public behavior interfaces are centralized in `internal/behavior`:

- `StepKind`
- `StepResult`
- `StepRef`
- `Step`
- `Timeline`
- `Signature`
- `SignatureOptions`
- `Divergence`
- `DivergenceKind`
- `Metrics`
- `MetricName`
- `ContextDisciplineMetrics`
- `ValidationMetrics`
- `EditScopeMetrics`
- `ToolEfficiencyMetrics`
- `RecoveryMetrics`
- `Extractor`
- `ExtractInput`
- `ExtractResult`
- `CommandKind`
- `CommandClassification`
- `CommandClassifier`
- `PathKind`
- `PathClassification`
- `PathClassifier`

The locked helper surface for module consumers:

- `func (s Step) StableKey() string`
- `func (s Step) IsValidation() bool`
- `func (s Step) IsEdit() bool`
- `func (s Step) IsFileRead() bool`
- `func SortFiles(files []string) []string`
- `func NormalizePathForKey(path string) string`
- `func IsKnownStepKind(kind StepKind) bool`
- `func IsAgentSpecificStepKind(kind StepKind) bool`

These interfaces must remain adapter-independent. They may depend on
`internal/trace`, but must not import `internal/adapters/**`.

## Module Boundaries

### 01-behavior-domain-model

Status: implemented in Round 0 / 01.

Allowed files:

- `internal/behavior/doc.go`
- `internal/behavior/step.go`
- `internal/behavior/result.go`
- `internal/behavior/signature.go`
- `internal/behavior/metrics.go`
- `internal/behavior/divergence.go`
- `internal/behavior/extractor.go`
- `internal/behavior/classifier.go`
- `internal/behavior/*_test.go`

Forbidden files:

- `internal/adapters/**`
- `internal/analysis/**`
- `internal/check/**`
- `internal/report/**`
- `internal/trace/**` core schema
- `cmd/**`

### 02-event-to-behavior-extractor

Allowed files:

- `internal/behavior/extractor.go`
- `internal/behavior/extractor_test.go`
- `internal/behavior/load.go`
- `internal/behavior/load_test.go`
- `testdata/behavior/runs/**`

Depends on:

- 01 domain model
- 03 command and path classifiers

Forbidden files:

- `internal/adapters/**`
- `internal/trace/**` core schema
- `cmd/**`

### 03-command-and-path-classifiers

Allowed files:

- `internal/behavior/command_classifier.go`
- `internal/behavior/path_classifier.go`
- `internal/behavior/classifier_test.go`
- `testdata/behavior/commands/**`
- `testdata/behavior/paths/**`

Depends on:

- 01 domain model

Forbidden files:

- `internal/adapters/**`
- `internal/trace/**`
- `cmd/**`

### 04-behavior-signature-and-cache

Allowed files:

- `internal/behavior/signature_builder.go`
- `internal/behavior/signature_key.go`
- `internal/behavior/signature_test.go`
- `internal/behavior/cache.go`, optional
- `internal/behavior/cache_test.go`, optional
- `testdata/behavior/golden/signature/**`

Depends on:

- 01 domain model
- 02 extractor
- 03 classifiers
- 06 metrics if metrics are computed during build

Forbidden files:

- `internal/adapters/**`
- `internal/trace/**` core schema
- `cmd/**`

### 05-first-behavior-divergence

Allowed files:

- `internal/behavior/divergence.go`
- `internal/behavior/diff.go`
- `internal/behavior/diff_test.go`
- `testdata/behavior/golden/diff/**`

Depends on:

- 01 domain model
- 04 signature
- 06 metrics for `MetricsDelta`, if implemented

Forbidden files:

- `internal/adapters/**`
- `internal/check/**`
- `internal/report/**`
- `cmd/**`

### 06-behavior-metrics

Allowed files:

- `internal/behavior/metrics.go`
- `internal/behavior/metrics_compute.go`
- `internal/behavior/metrics_test.go`
- `testdata/behavior/golden/metrics/**`

Depends on:

- 01 domain model
- 03 path classifier

Forbidden files:

- `internal/adapters/**`
- `internal/check/**`
- `internal/report/**`
- `cmd/**`

### 07-cli-integration-and-output

Allowed files:

- `cmd/agent-vcr/**`
- `internal/cli/behavior.go`
- `internal/cli/*_test.go`
- `internal/behavior/render.go`
- `internal/behavior/json.go`
- `internal/resolver/**`, only if needed for run resolution
- `testdata/behavior/golden/cli/**`

Depends on:

- 02 extractor
- 04 signature
- 05 divergence
- 06 metrics

Forbidden files:

- `internal/adapters/**`
- `internal/trace/**` core schema
- `internal/check/**`
- `internal/report/**`

### 08-testing-e2e-release

Allowed files:

- `scripts/e2e.ps1`
- `README.md`
- `docs/behavior-diff.md`
- `docs/trace-schema.md`
- `docs/release-checklist.md`
- `docs/v0.2-behavior-signature.md`
- `.github/workflows/ci.yml`
- `CHANGELOG.md`

Allowed only for small test-discovered fixes:

- `internal/**`
- `cmd/**`

Forbidden work:

- new features
- new adapters
- HarnessMetadata or HarnessDiff
- Matrix or Regression
- LLM Explain

## Parallel Plan

Round 1:

- `03-command-and-path-classifiers` can run after this Round 0 / 01.

Round 2:

- `02-event-to-behavior-extractor` can run after 03.
- `04-behavior-signature-and-cache` can start after 02 has a stable extractor
  API, or can prepare signature helpers that only depend on locked `Step`.

Round 3:

- `05-first-behavior-divergence` and `06-behavior-metrics` can run in parallel
  after 04 has the signature builder and 03 path classification is stable.

Round 4:

- `07-cli-integration-and-output` runs after 02, 04, 05, and 06.
- `08-testing-e2e-release` runs after 07.

Do not let multiple subagents modify `internal/behavior/step.go`,
`internal/behavior/signature.go`, `internal/behavior/metrics.go`, or
`internal/behavior/divergence.go` at the same time unless the main agent
explicitly coordinates an additive integration fix.

## Test Commands

Per behavior module:

```powershell
gofmt -w .
go test ./internal/behavior/...
go test ./...
```

Round integration:

```powershell
gofmt -w .
go test ./internal/behavior/...
go test ./...
go vet ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr --help
```

Final v0.2 validation after CLI and E2E:

```powershell
gofmt -w .
go test ./...
go vet ./...
go build ./cmd/agent-vcr
go run ./cmd/agent-vcr --help
go run ./cmd/agent-vcr behavior --help
powershell -ExecutionPolicy Bypass -File .\scripts\e2e.ps1
```

## Integration Rules

- `internal/behavior` must not import `internal/adapters/**`.
- `internal/behavior` may depend on normalized `internal/trace` types.
- Behavior StepKind values must remain generic and must not use prefixes such as
  `codex_`, `kimi_`, or `claude_`.
- Unknown or unclassified trace events should degrade to `raw_behavior` or
  `unknown` behavior, never panic.
- Behavior diff must ignore timestamps, event indexes, random ids, source event
  ids, absolute user home paths, and blob path differences.
- JSON output must stay stable enough to cache under
  `.agent-vcr/runs/<run-id>/behavior/`.
