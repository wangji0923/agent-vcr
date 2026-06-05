# 01 - Behavior Domain Model

## 目标

建立 v0.2 的核心领域模型：`behavior.Step`、`BehaviorSignature`、`BehaviorDiff`、`BehaviorMetrics` 的基础类型。此模块只定义类型、序列化、基础 helper，不做事件抽取、不做 diff 算法。

## 允许修改

```text
internal/behavior/step.go
internal/behavior/result.go
internal/behavior/signature.go
internal/behavior/metrics.go
internal/behavior/divergence.go
internal/behavior/doc.go
internal/behavior/*_test.go
```

## 禁止修改

```text
internal/adapters/**
internal/analysis/**
internal/check/**
internal/report/**
internal/trace/event.go 的既有语义
cmd/**
```

除非为了编译接线，不要修改 CLI。

## 数据结构

### StepKind

定义：

```go
type StepKind string
```

初始值：

```go
const (
    StepSearch            StepKind = "search"
    StepReadFile          StepKind = "read_file"
    StepInspectTest       StepKind = "inspect_test"
    StepEditFile          StepKind = "edit_file"
    StepRunTest           StepKind = "run_test"
    StepRunBuild          StepKind = "run_build"
    StepInstallDependency StepKind = "install_dependency"
    StepCallTool          StepKind = "call_tool"
    StepCallMCPTool       StepKind = "call_mcp_tool"
    StepPermissionRequest StepKind = "permission_request"
    StepRecoverFromError  StepKind = "recover_from_error"
    StepContextCompact    StepKind = "context_compact"
    StepProcessStart      StepKind = "process_start"
    StepProcessResult     StepKind = "process_result"
    StepRawBehavior       StepKind = "raw_behavior"
)
```

### StepResult

```go
type StepResult string

const (
    ResultUnknown StepResult = "unknown"
    ResultSuccess StepResult = "success"
    ResultFailure StepResult = "failure"
    ResultSkipped StepResult = "skipped"
)
```

### Step

```go
type Step struct {
    StepID         string            `json:"step_id"`
    RunID          string            `json:"run_id"`
    Index          int               `json:"index"`
    Kind           StepKind          `json:"kind"`
    Action         string            `json:"action,omitempty"`
    Target         string            `json:"target,omitempty"`
    Query          string            `json:"query,omitempty"`
    Command        string            `json:"command,omitempty"`
    ToolName       string            `json:"tool_name,omitempty"`
    Files          []string          `json:"files,omitempty"`
    Result         StepResult        `json:"result,omitempty"`
    SourceEventIDs []string          `json:"source_event_ids,omitempty"`
    Confidence     float64           `json:"confidence,omitempty"`
    Attributes     map[string]string `json:"attributes,omitempty"`
}
```

### Signature

```go
type Signature struct {
    SchemaVersion   string    `json:"schema_version"`
    RunID           string    `json:"run_id"`
    SourceTraceHash string    `json:"source_trace_hash,omitempty"`
    GeneratedAt     time.Time `json:"generated_at"`
    Steps           []Step    `json:"steps"`
    Metrics         Metrics   `json:"metrics"`
}
```

### Divergence

```go
type Divergence struct {
    Index       int     `json:"index"`
    Kind        string  `json:"kind"`
    RunAStep    *Step   `json:"run_a_step,omitempty"`
    RunBStep    *Step   `json:"run_b_step,omitempty"`
    Summary     string  `json:"summary"`
    Explanation string  `json:"explanation,omitempty"`
}
```

## 必须实现的 helper

```go
func (s Step) StableKey() string
func (s Step) IsValidation() bool
func (s Step) IsEdit() bool
func (s Step) IsFileRead() bool
func SortFiles(files []string) []string
func NormalizePathForKey(path string) string
```

`StableKey` 必须忽略：

```text
timestamp
absolute user home path
source_event_ids
blob path
step_id
index
```

## 测试要求

```text
TestStepJSONRoundTrip
TestStepStableKeyIgnoresIndexAndIDs
TestStepStableKeyNormalizesUserPaths
TestSortFilesStable
TestSignatureJSONRoundTrip
TestMetricsZeroValueJSON
```

## 验收命令

```powershell
gofmt -w .
go test ./internal/behavior/...
go test ./...
```

## Codex 执行提示词

```text
请只实现 01-behavior-domain-model.md。不要实现抽取器、diff、metrics 计算或 CLI。只定义 internal/behavior 的领域模型、稳定 key helper 和测试。完成后运行 go test ./internal/behavior/... 和 go test ./...。
```
