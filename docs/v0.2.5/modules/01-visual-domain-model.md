# 01 - Visual Domain Model

## 目标

建立 v0.2.5 可视化层的公共数据模型。这个模块只定义模型和基础校验，不实现加载、对齐、HTML 或 CLI。

## 允许修改

```text
internal/visualize/model.go
internal/visualize/validate.go
internal/visualize/model_test.go
testdata/visualize/model/**
```

## 禁止修改

```text
internal/adapters/**
internal/trace/** 核心 schema
internal/behavior/** public API，除非 additive 且标记 NEEDS_INTEGRATION
internal/cli/**
```

## 新增类型

### VisualReport

```go
type VisualReport struct {
    SchemaVersion string             `json:"schema_version"`
    GeneratedAt   time.Time          `json:"generated_at"`
    Runs          []VisualRun         `json:"runs"`
    Lanes         []BehaviorLane      `json:"lanes"`
    Alignment     []AlignmentRow      `json:"alignment"`
    Divergences   []VisualDivergence  `json:"divergences"`
    FileAccess    FileAccessCompare   `json:"file_access"`
    Metrics       []RunMetricsCards   `json:"metrics"`
    PathGraph     *PathGraph          `json:"path_graph,omitempty"`
    Warnings      []string            `json:"warnings,omitempty"`
}
```

### VisualRun

```go
type VisualRun struct {
    RunID     string     `json:"run_id"`
    Label     string     `json:"label"`
    Source    string     `json:"source"`
    Status    string     `json:"status"`
    StartedAt *time.Time `json:"started_at,omitempty"`
    EndedAt   *time.Time `json:"ended_at,omitempty"`
    StepCount int        `json:"step_count"`
}
```

### BehaviorLane / VisualStep

```go
type BehaviorLane struct {
    RunID string       `json:"run_id"`
    Label string       `json:"label"`
    Steps []VisualStep `json:"steps"`
}

type VisualStep struct {
    StepID      string            `json:"step_id"`
    Index       int               `json:"index"`
    Kind        string            `json:"kind"`
    Summary     string            `json:"summary"`
    Query       string            `json:"query,omitempty"`
    Command     string            `json:"command,omitempty"`
    Files       []string          `json:"files,omitempty"`
    Target      string            `json:"target,omitempty"`
    EventIDs    []string          `json:"event_ids,omitempty"`
    Significant bool              `json:"significant"`
    Divergent   bool              `json:"divergent"`
    Attributes  map[string]string `json:"attributes,omitempty"`
}
```

### AlignmentRow

```go
type AlignmentRow struct {
    RowIndex    int                 `json:"row_index"`
    Cells       map[string]StepCell `json:"cells"`
    IsDivergent bool                `json:"is_divergent"`
    Reason      string              `json:"reason,omitempty"`
}

type StepCell struct {
    RunID string      `json:"run_id"`
    Step  *VisualStep `json:"step,omitempty"`
    Gap   bool        `json:"gap"`
}
```

### VisualDivergence

```go
type VisualDivergence struct {
    BaselineRunID string      `json:"baseline_run_id"`
    CompareRunID  string      `json:"compare_run_id"`
    StepIndex     int         `json:"step_index"`
    Kind          string      `json:"kind"`
    Summary       string      `json:"summary"`
    Left          *VisualStep `json:"left,omitempty"`
    Right         *VisualStep `json:"right,omitempty"`
}
```

### File Access

```go
type FileAccessCompare struct {
    Rows []FileAccessRow `json:"rows"`
}

type FileAccessRow struct {
    Path string             `json:"path"`
    Runs map[string]FileUse `json:"runs"`
}

type FileUse struct {
    ReadCount   int    `json:"read_count"`
    EditCount   int    `json:"edit_count"`
    FirstStep   int    `json:"first_step"`
    LastStep    int    `json:"last_step"`
    FirstAction string `json:"first_action,omitempty"`
    LastAction  string `json:"last_action,omitempty"`
}
```

### Metrics Cards

```go
type RunMetricsCards struct {
    RunID string        `json:"run_id"`
    Cards []MetricsCard `json:"cards"`
}

type MetricsCard struct {
    Group string `json:"group"`
    Name  string `json:"name"`
    Value string `json:"value"`
    Level string `json:"level,omitempty"` // info | warn | bad | good
}
```

### PathGraph

```go
type PathGraph struct {
    Nodes []PathNode `json:"nodes"`
    Edges []PathEdge `json:"edges"`
}

type PathNode struct {
    ID      string   `json:"id"`
    Label   string   `json:"label"`
    Kind    string   `json:"kind"`
    RunIDs  []string `json:"run_ids"`
}

type PathEdge struct {
    From   string   `json:"from"`
    To     string   `json:"to"`
    RunIDs []string `json:"run_ids"`
}
```

## 校验函数

实现：

```go
func ValidateReport(report *VisualReport) error
func ValidateRunIDs(report *VisualReport) error
```

校验：

- run id 不能为空。
- lane run id 必须存在于 runs。
- alignment cells 的 run id 必须存在。
- `SchemaVersion` 非空。
- 多 run 数量建议 <= 5；超过时返回 warning 或 error，由 CLI 决定。

## 测试

必须覆盖：

- VisualReport JSON round-trip。
- VisualStep JSON round-trip。
- FileAccessCompare JSON round-trip。
- MetricsCard JSON round-trip。
- ValidateReport 成功 / 失败。
- 不 import adapters 架构测试。

## 验收命令

```powershell
gofmt -w .
go test ./internal/visualize/...
go test ./...
```
