# 02 - Visual Data Loader

## 目标

从 run id 加载 v0.2 behavior 数据，构造 `VisualRun`、`BehaviorLane` 和基础 `VisualStep`。

## 输入

优先读取：

```text
.agent-vcr/runs/<run-id>/behavior/signature.json
```

如果不存在：

```text
trace.ndjson → behavior extractor → behavior.Signature
```

## 允许修改

```text
internal/visualize/load.go
internal/visualize/convert.go
internal/visualize/load_test.go
testdata/visualize/load/**
```

## 禁止修改

```text
internal/adapters/**
internal/trace/** 核心 schema
internal/behavior/** public API，除非 additive
internal/cli/**
```

## 主要类型

```go
type LoadOptions struct {
    ProjectDir string
    RunIDs     []string
    NoCache    bool
    Labels     map[string]string
    MaxRuns    int
}

type LoadedRun struct {
    RunID     string
    Metadata  trace.Metadata
    Timeline  behavior.Timeline
    Signature behavior.Signature
    Metrics   behavior.Metrics
}
```

## 主要函数

```go
func LoadRuns(ctx context.Context, opts LoadOptions) ([]LoadedRun, error)
func BuildLane(run LoadedRun, label string) BehaviorLane
func VisualStepFromBehaviorStep(step behavior.Step) VisualStep
```

## 转换规则

`behavior.Step` → `VisualStep`：

- `Step.Kind` → `VisualStep.Kind`
- `Step.Summary` → `VisualStep.Summary`
- `Step.Query` → `VisualStep.Query`
- `Step.Command` → `VisualStep.Command`
- `Step.Files` → `VisualStep.Files`
- `Step.Target` → `VisualStep.Target`
- `Step.SourceRefs` → `VisualStep.EventIDs`
- `Step.Significant` → `VisualStep.Significant`

## 错误处理

- run 不存在：返回清晰错误。
- trace 损坏：返回错误，除非可以部分恢复。
- behavior cache 不存在：自动从 trace 重建。
- behavior cache 损坏：如果 `NoCache=false`，尝试从 trace 重建并记录 warning。
- metadata 缺失：降级生成 VisualRun，status 用 unknown。

## 测试

必须覆盖：

- 从 behavior cache 加载。
- cache 缺失时从 trace 重建。
- labels 生效。
- run 不存在时错误清晰。
- 多 run 加载顺序稳定。
- `MaxRuns` 限制。

## 验收命令

```powershell
gofmt -w .
go test ./internal/visualize/...
go test ./...
```
